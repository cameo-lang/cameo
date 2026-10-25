package aot

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/syntax"
)

// gen writes the Go of the compiled methods, from the types the check
// found, and records the arithmetic helpers and the big.Int constants they
// use.
type gen struct {
	b       *builder
	types   map[syntax.Node]typ
	results map[string]typ
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
	m     *method
	b     strings.Builder
	depth int
	temps int
	vars  map[string]typ
	big   bool
	// result is the type of the method's value.
	result typ
}

// goType is the Go type of a cameo type.
func (f *fn) goType(t typ) string {
	switch {
	case t == tBool:
		return "bool"
	case t == tFloat:
		return "float64"
	case t >= tClass:
		return "*object.Instance"
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

// function writes the method on int64, or its twin on *big.Int. A class's
// method takes the instance it runs on as self, before its parameters.
func (g *gen) function(m *method, big bool) string {
	d := m.def
	f := &fn{g: g, m: m, depth: 1, vars: map[string]typ{}, big: big, result: g.results[m.key]}
	var params []string
	if m.owner != nil {
		params = append(params, "self *object.Instance")
	}
	for i, p := range d.Params.Positional {
		f.vars[p.Name] = m.params[i]
		params = append(params, "v_"+p.Name+" "+f.goType(m.params[i]))
	}
	fmt.Fprintf(&f.b, "func %s%s(s *state%s) (%s, error) {\n", f.prefix(), m.goName(), prefixed(params), f.goType(f.result))
	if m.owner != nil {
		f.line("_ = self")
	}
	f.line("if s.stop.Load() {")
	f.line("\treturn %s, eval.ErrStopped", f.zero())
	f.line("}")
	if v := f.body(d.Body, true); v != "" {
		f.line("return %s, nil", v)
	}
	f.b.WriteString("}\n\n")
	return f.b.String()
}

// zero is the value a method returns beside an error.
func (f *fn) zero() string {
	if f.big && f.result == tInt || f.result >= tClass {
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
	return f.g.types[n]
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
	case *syntax.Float:
		return floatLit(n.Value)
	case *syntax.Self:
		return "self"
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
		stop := f.stopFlag()
		if n.DoWhile {
			// The test runs before every pass but the first, so a next,
			// which continues, still tests the condition.
			f.line("for first := true; ; first = false {")
		} else {
			f.line("for {")
		}
		f.depth++
		f.pass(stop)
		if n.DoWhile {
			f.line("if !first {")
			f.depth++
		}
		// until x is while !x, which leaves the loop when x holds.
		if not, ok := n.Cond.(*syntax.Not); ok {
			f.line("if %s {", f.expr(not.Value, true))
		} else {
			f.line("if !%s {", paren(f.expr(n.Cond, true)))
		}
		f.line("\tbreak")
		f.line("}")
		if n.DoWhile {
			f.depth--
			f.line("}")
		}
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
		return f.reach(n, f.g.b.bare(f.m.owner, n.Name), "self")
	}
	if k := f.g.b.news[n]; k != nil {
		return f.construct(n, k)
	}
	l := f.expr(n.Recv, true)
	lt := f.typeOf(n.Recv)
	if c := f.g.b.classOf(lt); c != nil {
		return f.reach(n, f.g.b.resolve(c, n.Name), l)
	}
	switch {
	case n.Name == "-@":
		if lt == tFloat {
			return "(-" + l + ")"
		}
		return f.arith("-@", l, "")
	case n.Name == "to_f":
		return f.toFloat(l, lt)
	case n.Name == "to_i":
		if lt == tInt {
			return l
		}
		return f.toInt(l)
	}
	r := f.expr(n.Args[0], true)
	rt := f.typeOf(n.Args[0])
	switch {
	case compare[n.Name] && lt != rt && numeric(lt) && numeric(rt):
		// Integer against Float compares exactly, as the interpreter does.
		t, ok := f.temp(), f.temp()
		f.line("%s, %s := object.CompareNumbers(%s, %s)", t, ok, f.value(l, lt), f.value(r, rt))
		if n.Name == "!=" {
			return fmt.Sprintf("!(%s && %s == 0)", ok, t)
		}
		return fmt.Sprintf("(%s && %s %s 0)", ok, t, n.Name)
	case compare[n.Name] && lt == tInt && f.big:
		return fmt.Sprintf("(%s.Cmp(%s) %s 0)", l, r, n.Name)
	case compare[n.Name]:
		return fmt.Sprintf("(%s %s %s)", l, n.Name, r)
	case lt == tFloat || rt == tFloat:
		l, r = f.toFloat(l, lt), f.toFloat(r, rt)
		if n.Name == "%" {
			f.g.helpers["fmod"] = true
			return fmt.Sprintf("fmod(%s, %s)", l, r)
		}
		// The conversion rounds the result, which keeps Go from fusing a
		// product and a sum into one operation with one rounding: the
		// interpreter rounds each.
		return fmt.Sprintf("float64(%s %s %s)", l, n.Name, r)
	}
	return f.arith(n.Name, l, r)
}

// reach writes a call that reaches a field or a candidate method: a field
// is read from the instance by its index, and leaves the call when it
// holds nil; a method is called directly, a class's with the instance
// first.
func (f *fn) reach(n *syntax.Call, t *target, inst string) string {
	if t.field != nil {
		h := map[typ]string{tFloat: "fieldFloat", tBool: "fieldBool"}[f.typeOf(n)]
		switch {
		case h != "":
		case f.typeOf(n) >= tClass:
			h = "fieldRef"
		case f.big:
			h = "fieldBig"
		default:
			h = "fieldInt"
		}
		f.g.helpers[h] = true
		tmp := f.temp()
		f.line("%s, err := %s(%s, %d)", tmp, h, inst, t.index)
		f.failOnErr()
		return tmp
	}
	var args []string
	if t.method.owner != nil {
		args = append(args, inst)
	}
	for _, a := range n.Args {
		args = append(args, f.expr(a, true))
	}
	tmp := f.temp()
	f.line("%s, err := %s%s(s%s)", tmp, f.prefix(), t.method.goName(), prefixed(args))
	f.failOnErr()
	return tmp
}

// construct writes Class.new: the arguments are read first, then, in a
// block of its own so that initialize's parameters shadow nothing, the
// fields' values go into a slice by index, and eval.Construct builds the
// instance as Class.new does, converting and checking each value.
func (f *fn) construct(n *syntax.Call, k *construction) string {
	var args []string
	for i, a := range n.Args {
		v := f.expr(a, true)
		switch {
		case k.params[i] == tFloat && f.typeOf(a) == tInt:
			v = f.toFloat(v, tInt)
		case k.params[i] == tInt:
			// A literal needs its type, since it becomes a local.
			v = f.typed(v)
		}
		args = append(args, v)
	}
	var values []string
	for _, kw := range n.KwArgs {
		values = append(values, f.expr(kw.Value, true))
	}
	res := f.temp()
	f.line("var %s *object.Instance", res)
	f.line("{")
	f.depth++
	fields := f.temp()
	f.line("%s := make([]object.Value, %d)", fields, len(k.cls.fields))
	outer := f.vars
	f.vars = map[string]typ{}
	if k.init != nil {
		for i, p := range k.init.Params.Positional {
			f.vars[p.Name] = k.params[i]
			f.line("v_%s := %s", p.Name, args[i])
			f.line("_ = v_%s", p.Name)
		}
		for _, set := range k.sets {
			i := k.cls.index[set.Name]
			f.line("%s[%d] = %s", fields, i, f.fieldValue(f.expr(set.Value, true), f.typeOf(set.Value), k.cls.fields[i]))
		}
	}
	for j, i := range k.keys {
		f.line("%s[%d] = %s", fields, i, f.fieldValue(values[j], f.typeOf(n.KwArgs[j].Value), k.cls.fields[i]))
	}
	f.vars = outer
	made := f.temp()
	f.line("%s, err := eval.Construct(s.ctx, s.class(%s), %s)", made, strconv.Quote(k.cls.name), fields)
	f.failOnErr()
	f.line("%s = %s", res, made)
	f.depth--
	f.line("}")
	return res
}

// fieldValue is the Value a construction gives a field: an Integer for a
// Float field converts, as the interpreter converts it.
func (f *fn) fieldValue(v string, t typ, fld *field) string {
	if fld.t == tFloat && fld.ref == "" && t == tInt {
		return f.value(f.toFloat(v, tInt), tFloat)
	}
	return f.value(v, t)
}

// value is the Value of a Go expression of type t, for a runtime call.
func (f *fn) value(v string, t typ) string {
	switch {
	case t == tFloat:
		return "object.Float(" + v + ")"
	case t == tBool:
		return "object.Bool(" + v + ")"
	case t >= tClass:
		return "object.InstanceValue(" + v + ")"
	case f.big:
		return "object.BigInt(" + v + ")"
	}
	return "object.Int(" + v + ")"
}

// toFloat converts an Integer to a Float, as Integer#to_f and mixed
// arithmetic do; a Float is itself.
func (f *fn) toFloat(v string, t typ) string {
	switch {
	case t == tFloat:
		return v
	case f.big:
		return "object.BigToFloat(" + v + ")"
	}
	return "float64(" + v + ")"
}

// toInt converts a Float to an Integer, as Float#to_i does: it leaves NaN
// and the infinities to the interpreter, which raises, and on int64 a
// value past 64 bits to the twin.
func (f *fn) toInt(v string) string {
	h := "toInt"
	if f.big {
		h = "bigToInt"
	}
	f.g.helpers[h] = true
	t := f.temp()
	f.line("%s, err := %s(%s)", t, h, v)
	f.failOnErr()
	return t
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
	stop := f.stopFlag()
	if f.big {
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
	f.pass(stop)
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

// stopFlag declares a local that holds the run's stop flag for a loop, so
// that a pass reads the flag alone and not the state first: the body may
// pass the state to a call, after which Go would read it again.
func (f *fn) stopFlag() string {
	stop := f.temp()
	f.line("%s := s.stop", stop)
	return stop
}

// pass writes the stop check of one loop pass: a read of the run's stop
// flag, which costs a load where reading the clock costs a call.
func (f *fn) pass(stop string) {
	f.line("if %s.Load() {", stop)
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
	return strings.HasPrefix(v, "v_") || v == "self" || v == "true" || v == "false" || strings.Trim(v, "-0123456789.e+") == ""
}

// floatLit is a Float literal as Go reads it: with a point or an exponent,
// so that it is a float64 and not an int.
func floatLit(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
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
	// Float#% takes the sign of the divisor, as Integer#% does; x % 0.0
	// is NaN, as math.Mod gives.
	"fmod": `func fmod(a, b float64) float64 {
	m := math.Mod(a, b)
	if m != 0 && (m < 0) != (b < 0) {
		m += b
	}
	return m
}

`,
	// Float#to_i truncates; NaN and the infinities raise FloatDomainError,
	// which is the interpreter's to do.
	"toInt": `func toInt(f float64) (int64, error) {
	if f != f || f >= 9223372036854775808.0 || f < -9223372036854775808.0 {
		return 0, eval.ErrDeopt
	}
	return int64(f), nil
}

`,
	"bigToInt": `func bigToInt(f float64) (*big.Int, error) {
	if f != f || f > math.MaxFloat64 || f < -math.MaxFloat64 {
		return nil, eval.ErrDeopt
	}
	i, _ := new(big.Float).SetFloat64(f).Int(nil)
	return i, nil
}

`,
	// classNamed reports whether c is the class of that name, or a class
	// under it, which is what an argument of a class's type must be.
	"classNamed": `func classNamed(c *object.Class, name string) bool {
	for _, a := range c.Ancestors() {
		if a.QualifiedName() == name {
			return true
		}
	}
	return false
}

`,
	// The field readers take the field by its index in the instance, which
	// its class fixes, the parent's fields first; a field that holds nil,
	// or an Integer past 64 bits where int64 is wanted, leaves the call.
	"fieldInt": `func fieldInt(inst *object.Instance, i int) (int64, error) {
	v, ok := inst.Fields[i].AsInt()
	if !ok {
		return 0, eval.ErrDeopt
	}
	return v, nil
}

`,
	"fieldBig": `func fieldBig(inst *object.Instance, i int) (*big.Int, error) {
	v, ok := inst.Fields[i].AsBigInt()
	if !ok {
		return nil, eval.ErrDeopt
	}
	return v, nil
}

`,
	"fieldFloat": `func fieldFloat(inst *object.Instance, i int) (float64, error) {
	v, ok := inst.Fields[i].AsFloat()
	if !ok {
		return 0, eval.ErrDeopt
	}
	return v, nil
}

`,
	"fieldBool": `func fieldBool(inst *object.Instance, i int) (bool, error) {
	v, ok := inst.Fields[i].AsBool()
	if !ok {
		return false, eval.ErrDeopt
	}
	return v, nil
}

`,
	"fieldRef": `func fieldRef(inst *object.Instance, i int) (*object.Instance, error) {
	v, ok := inst.Fields[i].AsInstance()
	if !ok {
		return nil, eval.ErrDeopt
	}
	return v, nil
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
