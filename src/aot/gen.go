package aot

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cameo-lang/cameo/src/syntax"
)

// gen writes the Go of the compiled methods, and records the arithmetic
// helpers they use.
type gen struct {
	helpers map[string]bool
}

// fn writes one method. Every expression becomes Go statements that leave
// its value in a Go expression without side effects: a constant, a local,
// a temporary, or a comparison of those.
type fn struct {
	g     *gen
	b     strings.Builder
	depth int
	temps int
	vars  map[string]typ
}

// goType is the Go type of a cameo type.
func goType(t typ) string {
	if t == tBool {
		return "bool"
	}
	return "int64"
}

// function writes fn_<name>, the method on int64.
func (g *gen) function(d *syntax.Def) string {
	f := &fn{g: g, depth: 1, vars: map[string]typ{}}
	var params []string
	for _, p := range d.Params.Positional {
		f.vars[p.Name] = tInt
		params = append(params, "v_"+p.Name+" int64")
	}
	fmt.Fprintf(&f.b, "func fn_%s(s *state%s) (int64, error) {\n", d.Name, prefixed(params))
	f.line("if s.stopped() {")
	f.line("\treturn 0, eval.ErrStopped")
	f.line("}")
	if v := f.body(d.Body, true); v != "" {
		f.line("return %s, nil", v)
	}
	f.b.WriteString("}\n\n")
	return f.b.String()
}

func (f *fn) line(format string, args ...any) {
	f.b.WriteString(strings.Repeat("\t", f.depth))
	fmt.Fprintf(&f.b, format, args...)
	f.b.WriteByte('\n')
}

func (f *fn) temp() string {
	f.temps++
	return fmt.Sprintf("t%d", f.temps)
}

// body writes a statement sequence and returns its last value, "" when it
// ends in a return or its value is not used.
func (f *fn) body(nodes []syntax.Node, value bool) string {
	for i, n := range nodes {
		last := i == len(nodes)-1
		if last && value {
			return f.expr(n, true)
		}
		if v := f.expr(n, false); v != "" && !plain(v) {
			f.line("_ = %s", v)
		}
		if f.typeOf(n) == tNever {
			return ""
		}
	}
	return ""
}

// typeOf is the type the check gave n.
func (f *fn) typeOf(n syntax.Node) typ {
	switch n := n.(type) {
	case *syntax.Bool, *syntax.Not, *syntax.And, *syntax.Or:
		return tBool
	case *syntax.Var:
		return f.vars[n.Name]
	case *syntax.Assign:
		return f.typeOf(n.Value)
	case *syntax.Seq:
		return f.bodyType(n.Body)
	case *syntax.Return:
		return tNever
	case *syntax.If:
		if n.Else == nil {
			return tInt
		}
		t, _ := join(f.bodyType(n.Then), f.bodyType(n.Else))
		return t
	case *syntax.Call:
		if n.Recv != nil && compare[n.Name] {
			return tBool
		}
	}
	return tInt
}

func (f *fn) bodyType(nodes []syntax.Node) typ {
	for _, n := range nodes {
		if t := f.typeOf(n); t == tNever {
			return tNever
		}
	}
	if len(nodes) == 0 {
		return tInt
	}
	return f.typeOf(nodes[len(nodes)-1])
}

// fail is the Go that leaves a method when a call it made failed.
const fail = "if err != nil {\n\treturn 0, err\n}"

func (f *fn) failOnErr() {
	for _, l := range strings.Split(fail, "\n") {
		f.line("%s", l)
	}
}

// expr writes n and returns the Go expression of its value, "" for a
// return.
func (f *fn) expr(n syntax.Node, value bool) string {
	switch n := n.(type) {
	case *syntax.Int:
		return fmt.Sprint(n.Value)
	case *syntax.Bool:
		return fmt.Sprint(n.Value)
	case *syntax.Var:
		return "v_" + n.Name
	case *syntax.Assign:
		v := f.expr(n.Value, true)
		if v == "" {
			return ""
		}
		if _, ok := f.vars[n.Name]; !ok {
			t := f.typeOf(n.Value)
			f.vars[n.Name] = t
			f.line("var v_%s %s", n.Name, goType(t))
			f.line("_ = v_%s", n.Name)
		}
		f.line("v_%s = %s", n.Name, v)
		return "v_" + n.Name
	case *syntax.Seq:
		return f.body(n.Body, value)
	case *syntax.Not:
		return "!" + paren(f.expr(n.Value, true))
	case *syntax.And:
		return f.logic(n.Left, n.Right, "")
	case *syntax.Or:
		return f.logic(n.Left, n.Right, "!")
	case *syntax.Return:
		v := f.expr(n.Value, true)
		if v != "" {
			f.line("return %s, nil", v)
		}
		return ""
	case *syntax.If:
		return f.ifExpr(n, value)
	case *syntax.While:
		f.line("for {")
		f.depth++
		f.line("if s.stopped() {")
		f.line("\treturn 0, eval.ErrStopped")
		f.line("}")
		// until x is while !x, which leaves the loop when x holds.
		if not, ok := n.Cond.(*syntax.Not); ok {
			f.line("if %s {", f.expr(not.Value, true))
		} else {
			f.line("if !%s {", paren(f.expr(n.Cond, true)))
		}
		f.line("\tbreak")
		f.line("}")
		f.body(n.Body, false)
		f.depth--
		f.line("}")
		return ""
	case *syntax.Call:
		return f.call(n)
	}
	panic(fmt.Sprintf("aot: %T passed the check and has no Go", n))
}

// logic writes && (neg "") or || (neg "!"): the right side runs only when
// the left one leaves the answer open.
func (f *fn) logic(l, r syntax.Node, neg string) string {
	t := f.temp()
	f.line("%s := %s", t, f.expr(l, true))
	f.line("if %s%s {", neg, t)
	f.depth++
	f.line("%s = %s", t, f.expr(r, true))
	f.depth--
	f.line("}")
	return t
}

func (f *fn) ifExpr(n *syntax.If, value bool) string {
	c := f.expr(n.Cond, true)
	t := ""
	if value {
		if ty := f.typeOf(n); ty != tNever {
			t = f.temp()
			f.line("var %s %s", t, goType(ty))
		}
	}
	branch := func(nodes []syntax.Node) {
		f.depth++
		if v := f.body(nodes, t != ""); v != "" {
			f.line("%s = %s", t, v)
		}
		f.depth--
	}
	f.line("if %s {", c)
	branch(n.Then)
	if len(n.Else) > 0 {
		f.line("} else {")
		branch(n.Else)
	}
	f.line("}")
	return t
}

// helperOf is the checked helper of an arithmetic operator.
var helperOf = map[string]string{"+": "add", "-": "sub", "*": "mul", "/": "div", "%": "mod", "-@": "neg"}

func (f *fn) call(n *syntax.Call) string {
	if n.Recv == nil {
		args := make([]string, len(n.Args))
		for i, a := range n.Args {
			args[i] = f.expr(a, true)
		}
		t := f.temp()
		f.line("%s, err := fn_%s(s%s)", t, n.Name, prefixed(args))
		f.failOnErr()
		return t
	}
	l := f.expr(n.Recv, true)
	if n.Name == "-@" {
		f.g.helpers["neg"] = true
		t := f.temp()
		f.line("%s, err := neg(%s)", t, l)
		f.failOnErr()
		return t
	}
	r := f.expr(n.Args[0], true)
	if h, ok := helperOf[n.Name]; ok {
		f.g.helpers[h] = true
		t := f.temp()
		f.line("%s, err := %s(%s, %s)", t, h, l, r)
		f.failOnErr()
		return t
	}
	return fmt.Sprintf("(%s %s %s)", l, n.Name, r)
}

// plain is a value Go lets a statement drop: a local, which its
// declaration already uses, or a literal. A temporary or an expression
// over temporaries has to be used.
func plain(v string) bool {
	return strings.HasPrefix(v, "v_") || v == "true" || v == "false" || strings.Trim(v, "-0123456789") == ""
}

// paren wraps an expression that is more than a name or a literal.
func paren(s string) string {
	if strings.ContainsAny(s, " ") && !(strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) {
		return "(" + s + ")"
	}
	return s
}

// helperSource is the Go of each helper: Integer arithmetic on int64 that
// leaves a result beyond 64 bits, and a division by zero, to the
// interpreter, which grows the Integer or raises.
var helperSource = map[string]string{
	"add": `func add(a, b int64) (int64, error) {
	c := a + b
	if (c > a) != (b > 0) {
		return 0, eval.ErrDeopt
	}
	return c, nil
}

`,
	"sub": `func sub(a, b int64) (int64, error) {
	c := a - b
	if (c < a) != (b > 0) {
		return 0, eval.ErrDeopt
	}
	return c, nil
}

`,
	"mul": `func mul(a, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	c := a * b
	if c/b != a || a == -1<<63 && b == -1 || b == -1<<63 && a == -1 {
		return 0, eval.ErrDeopt
	}
	return c, nil
}

`,
	// Integer#/ rounds towards negative infinity, where Go's / truncates.
	"div": `func div(a, b int64) (int64, error) {
	if b == 0 || a == -1<<63 && b == -1 {
		return 0, eval.ErrDeopt
	}
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q, nil
}

`,
	// Integer#% takes the sign of the divisor, where Go's % takes the
	// dividend's.
	"mod": `func mod(a, b int64) (int64, error) {
	if b == 0 {
		return 0, eval.ErrDeopt
	}
	r := a % b
	if r != 0 && (r < 0) != (b < 0) {
		r += b
	}
	return r, nil
}

`,
	"neg": `func neg(a int64) (int64, error) {
	if a == -1<<63 {
		return 0, eval.ErrDeopt
	}
	return -a, nil
}

`,
}

// walk calls visit on every node under n, n included.
func walk(n syntax.Node, visit func(syntax.Node)) {
	walkValue(reflect.ValueOf(n), visit)
}

func walkValue(v reflect.Value, visit func(syntax.Node)) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			walkValue(v.Elem(), visit)
		}
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		if n, ok := v.Interface().(syntax.Node); ok {
			visit(n)
		}
		walkValue(v.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				walkValue(v.Field(i), visit)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walkValue(v.Index(i), visit)
		}
	}
}
