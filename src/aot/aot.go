// Package aot compiles a cameo script's top-level methods to Go ahead of
// time: `cameo build`. It carries out the first steps of the plan in
// doc/design/aot.md, and compiles only what it can type without running
// the script, methods whose parameters are Integers and whose result is
// one, over Integer arithmetic, comparisons, locals, if, while, return,
// the block loops times, upto, downto and each, and calls of one another.
// The rest of the script runs on the interpreter, as it would without a
// build.
//
// A compiled method runs on int64, and has a twin on *big.Int. An Integer
// that outgrows 64 bits ends the int64 run, and the twin runs the call from
// the start. A division by zero, or an argument that is not an Integer,
// leaves the whole call to the interpreter, which runs the method's own
// body from the start and gives the answer the script would have given
// without a build. Starting over is safe because what the compiler accepts
// is pure: it computes a value and does nothing else.
package aot

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/syntax"
)

// Options says what Build writes.
type Options struct {
	// Package is the name of the generated package. "main", the default,
	// writes a program that runs the script; any other name writes a
	// package a host imports for its init, which registers the compiled
	// methods, and its Script constant, which it runs.
	Package string
}

// Result is what Build made of a script.
type Result struct {
	// Go is the source of the generated package, gofmt-formatted.
	Go []byte
	// Compiled names the methods that run as Go, sorted.
	Compiled []string
	// Left is why each top-level method that does not run as Go was left
	// to the interpreter, by name.
	Left map[string]string
}

// Build compiles the top-level methods of the script that it can.
func Build(file, src string, opts Options) (*Result, error) {
	prog, err := syntax.Parse(file, src)
	if err != nil {
		return nil, err
	}
	if opts.Package == "" {
		opts.Package = "main"
	}
	b := &builder{prog: prog, defs: map[string]*syntax.Def{}, left: map[string]string{}}
	b.collect()
	b.settle()
	res := &Result{Left: b.left}
	for name := range b.defs {
		res.Compiled = append(res.Compiled, name)
	}
	sort.Strings(res.Compiled)
	res.Go, err = b.emit(file, src, opts.Package, res.Compiled)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// builder holds the candidates: the top-level methods still believed to
// compile, which every check may call directly.
type builder struct {
	prog *syntax.Program
	defs map[string]*syntax.Def
	left map[string]string
}

// collect picks the candidates: top-level defs whose name no other def in
// the program has, anywhere, with required positional parameters only.
// Another def of the name, in a class say, could be what a call reaches,
// and the compiled call would skip it.
func (b *builder) collect() {
	count := map[string]int{}
	walk(b.prog, func(n syntax.Node) {
		if d, ok := n.(*syntax.Def); ok {
			count[d.Name]++
		}
	})
	for _, n := range b.prog.Body {
		d, ok := n.(*syntax.Def)
		if !ok {
			continue
		}
		switch {
		case count[d.Name] > 1:
			b.left[d.Name] = "the program defines it more than once"
		case !goName(d.Name):
			b.left[d.Name] = "its name is not a plain identifier"
		case reserved[d.Name]:
			b.left[d.Name] = "the interpreter calls it itself"
		case !plainParams(d.Params):
			b.left[d.Name] = "it takes optional, splat, keyword or block parameters"
		default:
			b.defs[d.Name] = d
		}
	}
}

// reserved are the names the interpreter reaches past dispatch.
var reserved = map[string]bool{"initialize": true, "method_missing": true, "respond_to_missing": true, "to_s": true, "inspect": true}

// settle drops the candidates that do not check, until the rest do: one
// that calls a dropped method no longer checks either.
func (b *builder) settle() {
	for changed := true; changed; {
		changed = false
		for name, d := range b.defs {
			if err := b.check(d); err != nil {
				b.left[name] = err.Error()
				delete(b.defs, name)
				changed = true
			}
		}
	}
}

func goName(s string) bool {
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return s != ""
}

func plainParams(p syntax.Params) bool {
	if len(p.Keywords) > 0 || p.KwRest != "" || p.BlockParam != "" {
		return false
	}
	for _, param := range p.Positional {
		if param.Splat || param.Default != nil || !goName(param.Name) {
			return false
		}
	}
	return true
}

// typ is the static type of an expression. never is the type of return,
// which leaves the method and has no value.
type typ int

const (
	tInt typ = iota
	tBool
	tNever
)

func (t typ) String() string {
	return [...]string{"Integer", "true or false", "nothing"}[t]
}

// join is the type of an if whose branches have types a and b.
func join(a, b typ) (typ, bool) {
	switch {
	case a == tNever:
		return b, true
	case b == tNever, a == b:
		return a, true
	}
	return 0, false
}

// scope is the locals of one method or block: their types, whether the
// check is inside an if or a loop, where a new local would be nil on the
// paths that skip it, and how many loops it is inside, which break and
// next need one of.
type scope struct {
	vars   map[string]typ
	nested int
	loops  int
}

// block is the scope of a block's body: it reads and sets the locals
// around it, and a local first set in it is its own, gone when it ends.
func (sc *scope) block(param string) *scope {
	child := &scope{vars: make(map[string]typ, len(sc.vars)+1), loops: sc.loops + 1}
	for k, v := range sc.vars {
		child.vars[k] = v
	}
	if param != "" {
		child.vars[param] = tInt
	}
	return child
}

func (b *builder) check(d *syntax.Def) error {
	sc := &scope{vars: map[string]typ{}}
	for _, p := range d.Params.Positional {
		sc.vars[p.Name] = tInt
	}
	t, err := b.body(d.Body, sc, true)
	if err != nil {
		return err
	}
	if t != tInt && t != tNever {
		return fmt.Errorf("its result is %s, not an Integer", t)
	}
	return nil
}

// body checks a statement sequence; value says whether its last value is
// used.
func (b *builder) body(nodes []syntax.Node, sc *scope, value bool) (typ, error) {
	if len(nodes) == 0 {
		if value {
			return 0, fmt.Errorf("an empty body is nil")
		}
		return tInt, nil
	}
	for i, n := range nodes {
		last := i == len(nodes)-1
		t, err := b.expr(n, sc, value && last)
		if err != nil {
			return 0, err
		}
		if t == tNever {
			if !last {
				return 0, fmt.Errorf("line %d: code follows a %s", n.Line(), describe(n))
			}
			return tNever, nil
		}
		if last {
			return t, nil
		}
	}
	panic("unreachable")
}

// expr checks one expression; value says whether its value is used.
func (b *builder) expr(n syntax.Node, sc *scope, value bool) (typ, error) {
	switch n := n.(type) {
	case *syntax.Int:
		return tInt, nil
	case *syntax.Bool:
		return tBool, nil
	case *syntax.Var:
		t, ok := sc.vars[n.Name]
		if !ok {
			return 0, fmt.Errorf("line %d: %s may be read before it is set", n.Line(), n.Name)
		}
		return t, nil
	case *syntax.Assign:
		t, err := b.expr(n.Value, sc, true)
		if err != nil {
			return 0, err
		}
		if t == tNever {
			return tNever, nil
		}
		old, ok := sc.vars[n.Name]
		switch {
		case !goName(n.Name):
			return 0, fmt.Errorf("line %d: the local %s has no Go name", n.Line(), n.Name)
		case ok && old != t:
			return 0, fmt.Errorf("line %d: %s holds %s and %s", n.Line(), n.Name, old, t)
		case !ok && sc.nested > 0:
			return 0, fmt.Errorf("line %d: %s is first set inside an if or a loop", n.Line(), n.Name)
		}
		sc.vars[n.Name] = t
		return t, nil
	case *syntax.Seq:
		return b.body(n.Body, sc, value)
	case *syntax.Not:
		return b.want(n.Value, sc, tBool, "the operand of !")
	case *syntax.And:
		return b.logic(n.Left, n.Right, sc)
	case *syntax.Or:
		return b.logic(n.Left, n.Right, sc)
	case *syntax.Return:
		if n.Value == nil {
			return 0, fmt.Errorf("line %d: return without a value returns nil", n.Line())
		}
		if _, err := b.want(n.Value, sc, tInt, "the value of return"); err != nil {
			return 0, err
		}
		return tNever, nil
	case *syntax.If:
		if _, err := b.want(n.Cond, sc, tBool, "the condition"); err != nil {
			return 0, err
		}
		if n.Else == nil && value {
			return 0, fmt.Errorf("line %d: an if without an else is nil when its condition is false", n.Line())
		}
		sc.nested++
		defer func() { sc.nested-- }()
		then, err := b.body(n.Then, sc, value)
		if err != nil {
			return 0, err
		}
		if !value {
			if _, err := b.body(n.Else, sc, false); err != nil {
				return 0, err
			}
			return tInt, nil
		}
		other, err := b.body(n.Else, sc, true)
		if err != nil {
			return 0, err
		}
		t, ok := join(then, other)
		if !ok {
			return 0, fmt.Errorf("line %d: the branches give %s and %s", n.Line(), then, other)
		}
		return t, nil
	case *syntax.While:
		if value {
			return 0, fmt.Errorf("line %d: the value of a while is nil", n.Line())
		}
		sc.nested++
		sc.loops++
		defer func() { sc.nested--; sc.loops-- }()
		if _, err := b.want(n.Cond, sc, tBool, "the condition"); err != nil {
			return 0, err
		}
		if _, err := b.body(n.Body, sc, false); err != nil {
			return 0, err
		}
		return tInt, nil
	case *syntax.Break:
		return b.leave(n, n.Value, sc, "break")
	case *syntax.Next:
		return b.leave(n, n.Value, sc, "next")
	case *syntax.Call:
		return b.call(n, sc, value)
	}
	return 0, fmt.Errorf("line %d: %s is not compiled yet", n.Line(), describe(n))
}

// leave checks a break or a next, which ends the innermost loop or its
// pass and has no value of its own.
func (b *builder) leave(n syntax.Node, value syntax.Node, sc *scope, what string) (typ, error) {
	if sc.loops == 0 {
		return 0, fmt.Errorf("line %d: %s outside a loop or a block", n.Line(), what)
	}
	if value != nil {
		return 0, fmt.Errorf("line %d: %s with a value is not compiled yet", n.Line(), what)
	}
	return tNever, nil
}

func (b *builder) want(n syntax.Node, sc *scope, want typ, what string) (typ, error) {
	t, err := b.expr(n, sc, true)
	if err != nil {
		return 0, err
	}
	if t != want && t != tNever {
		return 0, fmt.Errorf("line %d: %s is %s, not %s", n.Line(), what, t, want)
	}
	return want, nil
}

func (b *builder) logic(l, r syntax.Node, sc *scope) (typ, error) {
	if _, err := b.want(l, sc, tBool, "an operand of && or ||"); err != nil {
		return 0, err
	}
	// The right side runs on some paths only, as a branch does.
	sc.nested++
	defer func() { sc.nested-- }()
	return b.want(r, sc, tBool, "an operand of && or ||")
}

// arith and compare are the Integer operators the compiler knows: the
// methods of Integer, which a script cannot redefine.
var arith = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true}
var compare = map[string]bool{"<": true, "<=": true, ">": true, ">=": true, "==": true, "!=": true}

// loops are the methods that call a block once per Integer, which compile
// to a Go loop with the block's body inside: times, upto and downto on an
// Integer, each on a Range of Integers.
var loops = map[string]bool{"times": true, "upto": true, "downto": true, "each": true}

func (b *builder) call(n *syntax.Call, sc *scope, value bool) (typ, error) {
	if len(n.KwArgs) > 0 || n.BlockArg != nil || n.Safe {
		return 0, fmt.Errorf("line %d: the call of %s takes keywords or a block argument", n.Line(), n.Name)
	}
	for _, a := range n.Args {
		if _, ok := a.(*syntax.Splat); ok {
			return 0, fmt.Errorf("line %d: the call of %s splats its arguments", n.Line(), n.Name)
		}
	}
	if n.Block != nil {
		if n.Recv == nil || !loops[n.Name] {
			return 0, fmt.Errorf("line %d: the block of %s is not compiled yet", n.Line(), n.Name)
		}
		return b.loop(n, sc, value)
	}
	if n.Recv == nil {
		d, ok := b.defs[n.Name]
		if !ok {
			why := "it is not a top-level method of the script"
			if _, isLeft := b.left[n.Name]; isLeft {
				why = "it is left to the interpreter"
			}
			return 0, fmt.Errorf("line %d: it calls %s, and %s", n.Line(), n.Name, why)
		}
		if len(n.Args) != len(d.Params.Positional) {
			return 0, fmt.Errorf("line %d: it calls %s with %d arguments, not %d", n.Line(), n.Name, len(n.Args), len(d.Params.Positional))
		}
		for _, a := range n.Args {
			if _, err := b.want(a, sc, tInt, "an argument of "+n.Name); err != nil {
				return 0, err
			}
		}
		return tInt, nil
	}
	recv, err := b.expr(n.Recv, sc, true)
	if err != nil {
		return 0, err
	}
	switch {
	case n.Name == "-@" && len(n.Args) == 0 && recv == tInt:
		return tInt, nil
	case len(n.Args) == 1 && (arith[n.Name] || compare[n.Name]):
		arg, err := b.expr(n.Args[0], sc, true)
		if err != nil {
			return 0, err
		}
		switch {
		case recv == tInt && arg == tInt && arith[n.Name]:
			return tInt, nil
		case recv == tInt && arg == tInt:
			return tBool, nil
		case recv == tBool && arg == tBool && (n.Name == "==" || n.Name == "!="):
			return tBool, nil
		}
		return 0, fmt.Errorf("line %d: %s %s %s is not compiled", n.Line(), recv, n.Name, arg)
	}
	return 0, fmt.Errorf("line %d: it calls %s on %s", n.Line(), n.Name, recv)
}

// loop checks a call of times, upto, downto or each with a block: Integer
// bounds, one Integer parameter at most, and a body that is a statement.
// The value of times, upto and downto is the receiver; each returns its
// Range, so it compiles as a statement only.
func (b *builder) loop(n *syntax.Call, sc *scope, value bool) (typ, error) {
	p := n.Block.Params
	if len(p.Keywords) > 0 || p.KwRest != "" || p.BlockParam != "" || len(p.Positional) > 1 || len(p.Positional) == 1 && (p.Positional[0].Splat || p.Positional[0].Default != nil || !goName(p.Positional[0].Name)) {
		return 0, fmt.Errorf("line %d: the block of %s takes more than one plain parameter", n.Line(), n.Name)
	}
	var bounds []syntax.Node
	switch {
	case n.Name == "each":
		r, ok := n.Recv.(*syntax.Range)
		if !ok {
			t, err := b.expr(n.Recv, sc, true)
			if err != nil {
				return 0, err
			}
			return 0, fmt.Errorf("line %d: it calls each on %s", n.Line(), t)
		}
		if value {
			return 0, fmt.Errorf("line %d: the value of each is a Range", n.Line())
		}
		if len(n.Args) != 0 {
			return 0, fmt.Errorf("line %d: each takes no arguments", n.Line())
		}
		bounds = []syntax.Node{r.Lo, r.Hi}
	case n.Name == "times" && len(n.Args) == 0, n.Name != "times" && len(n.Args) == 1:
		bounds = append([]syntax.Node{n.Recv}, n.Args...)
	default:
		return 0, fmt.Errorf("line %d: %s takes %d arguments", n.Line(), n.Name, len(n.Args))
	}
	for _, bound := range bounds {
		if _, err := b.want(bound, sc, tInt, "a bound of "+n.Name); err != nil {
			return 0, err
		}
	}
	param := ""
	if len(p.Positional) == 1 {
		param = p.Positional[0].Name
	}
	if _, err := b.body(n.Block.Body, sc.block(param), false); err != nil {
		return 0, err
	}
	return tInt, nil
}

// describe names a node for a message: "a String" for a Str.
func describe(n syntax.Node) string {
	name := strings.TrimPrefix(fmt.Sprintf("%T", n), "*syntax.")
	if d, ok := descriptions[name]; ok {
		return d
	}
	return "the construct " + name
}

var descriptions = map[string]string{
	"Nil": "nil", "Float": "a Float", "BigInt": "an Integer beyond 64 bits",
	"Str": "a String", "Interp": "an interpolated String", "Sym": "a Symbol",
	"Regexp": "a Regexp", "List": "an Array", "Map": "a Hash", "Range": "a Range",
	"Self": "self", "Const": "a constant", "ScopedConst": "a constant",
	"Lambda": "a lambda", "Yield": "yield", "Super": "super", "Break": "break",
	"Next": "next", "Return": "return", "Case": "case", "CaseIn": "case/in", "Begin": "begin/rescue",
	"Def": "a def", "ClassDef": "a class", "MultiAssign": "a multiple assignment",
	"MutCall": "a call that changes its receiver", "Async": "async", "Await": "await",
}

// emit writes the Go source.
func (b *builder) emit(file, src, pkg string, names []string) ([]byte, error) {
	g := &gen{helpers: map[string]bool{}, consts: map[int64]string{}}
	var fns bytes.Buffer
	for _, name := range names {
		fns.WriteString(g.function(b.defs[name], false))
		fns.WriteString(g.function(b.defs[name], true))
		fns.WriteString(entry(b.defs[name]))
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by cameo build from %s; DO NOT EDIT.\n\n", baseName(file))
	fmt.Fprintf(&out, "package %s\n\n", pkg)
	out.WriteString("import (\n")
	if pkg == "main" {
		out.WriteString("\t\"fmt\"\n")
	}
	if len(names) > 0 {
		out.WriteString("\t\"math/big\"\n")
	}
	if pkg == "main" {
		out.WriteString("\t\"os\"\n\t\"runtime/debug\"\n\n\t\"github.com/cameo-lang/cameo/src/cameo\"\n")
	}
	out.WriteString("\t\"github.com/cameo-lang/cameo/src/eval\"\n\t\"github.com/cameo-lang/cameo/src/object\"\n)\n\n")
	if pkg == "main" {
		fmt.Fprintf(&out, "// file names the script in its error messages.\nconst file = %s\n\n", strconv.Quote(baseName(file)))
	} else {
		fmt.Fprintf(&out, "// File names the script in its error messages.\nconst File = %s\n\n", strconv.Quote(baseName(file)))
	}
	script := "script"
	if pkg != "main" {
		script = "Script"
	}
	fmt.Fprintf(&out, "// %s is the source the methods below were compiled from. They run\n// only for a program parsed from it.\nconst %s = %s\n\n", script, script, goString(src))
	fmt.Fprintf(&out, "// digest identifies the program: syntax.Digest of %s, parsed.\nconst digest = %s\n\n", script, strconv.Quote(syntax.Digest(b.prog)))
	out.WriteString("func init() {\n\teval.RegisterCompiled(digest, eval.Compiled{\n")
	for _, name := range names {
		fmt.Fprintf(&out, "\t\t%s: call_%s,\n", strconv.Quote(name), name)
	}
	out.WriteString("\t})\n}\n\n")
	if pkg == "main" {
		out.WriteString(mainFunc)
	}
	out.WriteString(stateType)
	out.WriteString(g.constants())
	out.Write(fns.Bytes())
	for _, h := range sortedKeys(g.helpers) {
		out.WriteString(helperSource[h])
	}
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("aot: the generated Go does not parse: %v\n%s", err, out.Bytes())
	}
	return formatted, nil
}

// goString is src as a Go string literal: a raw one, which reads as the
// script does, unless src holds a backquote or a carriage return.
func goString(src string) string {
	if strings.ContainsAny(src, "`\r") {
		return strconv.Quote(src)
	}
	return "`" + src + "`"
}

func baseName(file string) string {
	if i := strings.LastIndexByte(file, '/'); i >= 0 {
		return file[i+1:]
	}
	return file
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mainFunc runs the script as the cameo command does, with the same
// collector settings.
const mainFunc = `func main() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(1 << 30)
	}
	if err := cameo.Run(file, script, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

`

// stateType is what every compiled call carries: the run it belongs to,
// and a count of calls and loop passes, so that the run's stop is checked
// every few thousand.
const stateType = `// state is one call from the interpreter into compiled code: the run it
// belongs to, which may be stopped, and the calls made since.
type state struct {
	ctx   object.Context
	calls uint32
}

// stopped counts a call or a loop pass, and every 4096 of them reports
// whether the run was stopped, which the interpreter checks at each one.
func (s *state) stopped() bool {
	s.calls++
	return s.calls&4095 == 0 && eval.Stopped(s.ctx)
}

`

// entry is the method the interpreter calls: it takes the arguments as
// Values, runs on int64 while the Integers fit, on *big.Int once one does
// not, and leaves any call it cannot run to the interpreter.
func entry(d *syntax.Def) string {
	var b strings.Builder
	name := d.Name
	n := len(d.Params.Positional)
	fmt.Fprintf(&b, "// call_%s is %s as a method: it runs with Integer arguments and no\n// block, on int64 while the Integers fit and on *big.Int once one does not,\n// and leaves every other call to the interpreter.\n", name, name)
	fmt.Fprintf(&b, "func call_%s(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {\n", name)
	fmt.Fprintf(&b, "\tif len(args) != %d || kw != nil || !(block.IsZero() || block.IsNil()) {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n", n)
	b.WriteString("\ts := &state{ctx: ctx}\n")
	var small, large, fits []string
	for i := range d.Params.Positional {
		fmt.Fprintf(&b, "\ta%d, ok%d := args[%d].AsInt()\n", i, i, i)
		small = append(small, fmt.Sprintf("a%d", i))
		large = append(large, fmt.Sprintf("b%d", i))
		fits = append(fits, fmt.Sprintf("ok%d", i))
	}
	if n == 0 {
		b.WriteString("\t_ = args\n")
		fits = append(fits, "true")
	}
	fmt.Fprintf(&b, "\tif %s {\n", strings.Join(fits, " && "))
	fmt.Fprintf(&b, "\t\tr, err := fn_%s(s%s)\n", name, prefixed(small))
	b.WriteString("\t\tif err != eval.ErrDeopt {\n\t\t\tif err != nil {\n\t\t\t\treturn object.Value{}, err\n\t\t\t}\n\t\t\treturn object.Int(r), nil\n\t\t}\n\t}\n")
	for i := range d.Params.Positional {
		fmt.Fprintf(&b, "\tb%d, ok := args[%d].AsBigInt()\n\tif !ok {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n", i, i)
	}
	fmt.Fprintf(&b, "\tr, err := big_%s(s%s)\n", name, prefixed(large))
	b.WriteString("\tif err != nil {\n\t\treturn object.Value{}, err\n\t}\n\treturn object.BigInt(r), nil\n}\n\n")
	return b.String()
}

func prefixed(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return ", " + strings.Join(args, ", ")
}
