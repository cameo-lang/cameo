package aot

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/cameo-lang/cameo/src/syntax"
)

// gen writes the Go of the compiled methods, and records the arithmetic
// helpers and the big.Int constants they use.
type gen struct {
	helpers map[string]bool
	consts  map[int64]string
}

// fn writes one method, on int64 or, when big is set, its twin on
// *big.Int, which runs a call whose Integers outgrow 64 bits. Every
// expression becomes Go statements that leave its value in a Go expression
// without side effects: a constant, a local, a temporary, or a comparison
// of those.
type fn struct {
	g     *gen
	b     strings.Builder
	depth int
	temps int
	vars  map[string]typ
	big   bool
	// counters are the pass counters declared by counter, which pass
	// increments; a loop variable is not one.
	counters map[string]bool
}

// goType is the Go type of a cameo type.
func (f *fn) goType(t typ) string {
	switch {
	case t == tBool:
		return "bool"
	case f.big:
		return "*big.Int"
	}
	return "int64"
}

// prefix is the prefix of the method's Go name: fn_ on int64, big_ on
// *big.Int.
func (f *fn) prefix() string {
	if f.big {
		return "big_"
	}
	return "fn_"
}

// function writes the method on int64, or its twin on *big.Int.
func (g *gen) function(d *syntax.Def, big bool) string {
	f := &fn{g: g, depth: 1, vars: map[string]typ{}, big: big, counters: map[string]bool{}}
	var params []string
	for _, p := range d.Params.Positional {
		f.vars[p.Name] = tInt
		params = append(params, "v_"+p.Name+" "+f.goType(tInt))
	}
	fmt.Fprintf(&f.b, "func %s%s(s *state%s) (%s, error) {\n", f.prefix(), d.Name, prefixed(params), f.goType(tInt))
	f.line("if s.stopped() {")
	f.line("\treturn %s, eval.ErrStopped", f.zero())
	f.line("}")
	if v := f.body(d.Body, true); v != "" {
		f.line("return %s, nil", v)
	}
	f.b.WriteString("}\n\n")
	return f.b.String()
}

// zero is the Integer a method returns beside an error.
func (f *fn) zero() string {
	if f.big {
		return "nil"
	}
	return "0"
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
	case *syntax.Return, *syntax.Break, *syntax.Next:
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

// failOnErr writes the Go that leaves a method when a call it made failed.
func (f *fn) failOnErr() {
	f.line("if err != nil {")
	f.line("\treturn %s, err", f.zero())
	f.line("}")
}

// expr writes n and returns the Go expression of its value, "" for a
// return.
func (f *fn) expr(n syntax.Node, value bool) string {
	switch n := n.(type) {
	case *syntax.Int:
		if f.big {
			return f.g.constant(n.Value)
		}
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
			f.line("var v_%s %s", n.Name, f.goType(t))
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
		c := f.counter()
		f.line("for {")
		f.depth++
		f.pass(c)
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
	case *syntax.Break:
		f.line("break")
		return ""
	case *syntax.Next:
		f.line("continue")
		return ""
	case *syntax.Call:
		return f.call(n, value)
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
			f.line("var %s %s", t, f.goType(ty))
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

// helperOf is the checked helper of an arithmetic operator on int64; on
// *big.Int, bigOp is the big.Int method of an operator that cannot fail,
// and bigHelperOf the helper of a division, which can.
var helperOf = map[string]string{"+": "add", "-": "sub", "*": "mul", "/": "div", "%": "mod", "-@": "neg"}
var bigOp = map[string]string{"+": "Add", "-": "Sub", "*": "Mul", "-@": "Neg"}
var bigHelperOf = map[string]string{"/": "bigDiv", "%": "bigMod"}

func (f *fn) call(n *syntax.Call, value bool) string {
	if n.Block != nil {
		return f.loop(n, value)
	}
	if n.Recv == nil {
		args := make([]string, len(n.Args))
		for i, a := range n.Args {
			args[i] = f.expr(a, true)
		}
		t := f.temp()
		f.line("%s, err := %s%s(s%s)", t, f.prefix(), n.Name, prefixed(args))
		f.failOnErr()
		return t
	}
	l := f.expr(n.Recv, true)
	if n.Name == "-@" {
		return f.arith("-@", l, "")
	}
	r := f.expr(n.Args[0], true)
	if _, ok := helperOf[n.Name]; ok {
		return f.arith(n.Name, l, r)
	}
	if f.big && f.typeOf(n.Recv) == tInt {
		return fmt.Sprintf("(%s.Cmp(%s) %s 0)", l, r, n.Name)
	}
	return fmt.Sprintf("(%s %s %s)", l, n.Name, r)
}

// loop writes times, upto, downto or each with a block as a Go loop over
// the block's body. The bounds are read once, before the loop, as the
// methods read them; the counter is the loop's own, so that the body may
// set the parameter without moving the loop; and a local the body first
// sets is declared in the body, new on every pass, as a block's locals
// are. On int64 a loop up to the largest Integer, or down to the smallest,
// is left to the twin on *big.Int, where the counter cannot wrap.
func (f *fn) loop(n *syntax.Call, value bool) string {
	var from, to syntax.Node
	cmp, step, edge := "<=", "Add", "1<<63 - 1"
	switch n.Name {
	case "times":
		from, to, cmp = nil, n.Recv, "<"
	case "upto":
		from, to = n.Recv, n.Args[0]
	case "downto":
		from, to, cmp, step, edge = n.Recv, n.Args[0], ">=", "Sub", "-1 << 63"
	case "each":
		r := n.Recv.(*syntax.Range)
		from, to = r.Lo, r.Hi
		if r.Exclusive {
			cmp = "<"
		}
	}
	lo := "int64(0)"
	if f.big {
		lo = "new(big.Int)"
	}
	if from != nil {
		lo = f.temp()
		f.line("%s := %s", lo, f.typed(f.expr(from, true)))
	}
	hi := f.temp()
	f.line("%s := %s", hi, f.typed(f.expr(to, true)))
	i := f.temp()
	// A loop on int64 counts its passes on its own variable; one on
	// *big.Int, and a while, on a counter of its own.
	c := i
	if f.big {
		c = f.counter()
		f.g.constant(1)
		if from != nil {
			lo = fmt.Sprintf("new(big.Int).Set(%s)", lo)
		}
		f.line("for %s := %s; %s.Cmp(%s) %s 0; %s = new(big.Int).%s(%s, big_1) {", i, lo, i, hi, cmp, i, step, i)
	} else {
		if cmp != "<" {
			f.line("if %s == %s {", hi, edge)
			f.line("\treturn 0, eval.ErrDeopt")
			f.line("}")
		}
		next := "++"
		if step == "Sub" {
			next = "--"
		}
		f.line("for %s := %s; %s %s %s; %s%s {", i, lo, i, cmp, hi, i, next)
	}
	f.depth++
	f.pass(c)
	outer := f.vars
	f.vars = make(map[string]typ, len(outer)+1)
	for k, v := range outer {
		f.vars[k] = v
	}
	if p := n.Block.Params.Positional; len(p) == 1 {
		f.vars[p[0].Name] = tInt
		f.line("v_%s := %s", p[0].Name, i)
		f.line("_ = v_%s", p[0].Name)
	}
	f.body(n.Block.Body, false)
	f.vars = outer
	f.depth--
	f.line("}")
	if !value {
		return ""
	}
	// times, upto and downto return their receiver.
	if n.Name == "times" {
		return hi
	}
	return lo
}

// counter declares the pass counter of a loop that has no variable of its
// own to count on: a local, which stays in a register, where the state's
// call counter is a store on every pass.
func (f *fn) counter() string {
	c := f.temp()
	f.counters[c] = true
	f.line("var %s uint32", c)
	return c
}

// pass writes the stop check of one loop pass: every 4096 passes, by the
// low bits of the loop's variable or of its counter, a read of the run's
// stop. A counter is incremented first; a loop variable moves on its own.
func (f *fn) pass(c string) {
	if f.counters[c] {
		f.line("%s++", c)
	}
	f.line("if %s&4095 == 0 && eval.Stopped(s.ctx) {", c)
	f.line("\treturn %s, eval.ErrStopped", f.zero())
	f.line("}")
}

// typed gives an Integer literal the type of the method's Integers, which
// a Go loop header needs; any other expression has it.
func (f *fn) typed(v string) string {
	if !f.big && strings.Trim(v, "-0123456789") == "" {
		return "int64(" + v + ")"
	}
	return v
}

// arith writes an arithmetic operator: on int64 a checked helper, which
// fails past 64 bits; on *big.Int a method of big.Int, or a helper for a
// division, which fails on a zero divisor.
func (f *fn) arith(op, l, r string) string {
	t := f.temp()
	switch {
	case f.big && bigOp[op] != "":
		if r == "" {
			f.line("%s := new(big.Int).%s(%s)", t, bigOp[op], l)
		} else {
			f.line("%s := new(big.Int).%s(%s, %s)", t, bigOp[op], l, r)
		}
		return t
	case f.big:
		h := bigHelperOf[op]
		f.g.helpers[h] = true
		f.g.constant(1)
		f.line("%s, err := %s(%s, %s)", t, h, l, r)
	case r == "":
		f.g.helpers["neg"] = true
		f.line("%s, err := neg(%s)", t, l)
	default:
		h := helperOf[op]
		f.g.helpers[h] = true
		f.line("%s, err := %s(%s, %s)", t, h, l, r)
	}
	f.failOnErr()
	return t
}

// constant is the package-level big.Int of an Integer literal, which the
// twins share and never change.
func (g *gen) constant(v int64) string {
	if name, ok := g.consts[v]; ok {
		return name
	}
	name := "big_" + strings.ReplaceAll(fmt.Sprint(v), "-", "m")
	g.consts[v] = name
	return name
}

// constants writes the big.Int constants, sorted by value.
func (g *gen) constants() string {
	if len(g.consts) == 0 {
		return ""
	}
	values := make([]int64, 0, len(g.consts))
	for v := range g.consts {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	var b strings.Builder
	b.WriteString("// The Integer literals of the methods on *big.Int, which they share and\n// never change.\nvar (\n")
	for _, v := range values {
		fmt.Fprintf(&b, "\t%s = big.NewInt(%d)\n", g.consts[v], v)
	}
	b.WriteString(")\n\n")
	return b.String()
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
// leaves a result beyond 64 bits to the twin on *big.Int, and a division
// by zero, in either form, to the interpreter, which raises.
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
	"bigDiv": `func bigDiv(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, eval.ErrDeopt
	}
	q, m := new(big.Int).QuoRem(a, b, new(big.Int))
	if m.Sign() != 0 && (m.Sign() < 0) != (b.Sign() < 0) {
		q.Sub(q, big_1)
	}
	return q, nil
}

`,
	"bigMod": `func bigMod(a, b *big.Int) (*big.Int, error) {
	if b.Sign() == 0 {
		return nil, eval.ErrDeopt
	}
	m := new(big.Int).Rem(a, b)
	if m.Sign() != 0 && (m.Sign() < 0) != (b.Sign() < 0) {
		m.Add(m, b)
	}
	return m, nil
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
