// Package aot compiles a cameo script's methods to Go ahead of time:
// `cameo build`. It carries out the first steps of the plan in
// doc/design/aot.md, and compiles only what it can type without running
// the script: top-level methods, and the methods of top-level classes with
// typed fields, whose parameters are Integers or instances of those
// classes and whose result is an Integer or a Float, over Integer and Float
// arithmetic, comparisons, locals, if, while, return, the block loops
// times, upto, downto and each, reads of typed fields, and calls of one
// another. The rest of the script runs on the interpreter, as it would
// without a build.
//
// A compiled method runs on int64, and has a twin on *big.Int. An Integer
// that outgrows 64 bits ends the int64 run, and the twin runs the call from
// the start. A division by zero, an argument that is not what the method
// takes, or a field that holds nil, leaves the whole call to the
// interpreter, which runs the method's own body from the start and gives
// the answer the script would have given without a build. Starting over is
// safe because what the compiler accepts is pure: it computes a value and
// does nothing else.
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
	// Compiled names the methods that run as Go, sorted: a top-level method
	// by its name, a class's as Class#name.
	Compiled []string
	// Left is why each method that does not run as Go was left to the
	// interpreter, by name.
	Left map[string]string
}

// Build compiles the methods of the script that it can.
func Build(file, src string, opts Options) (*Result, error) {
	prog, err := syntax.Parse(file, src)
	if err != nil {
		return nil, err
	}
	if opts.Package == "" {
		opts.Package = "main"
	}
	b := &builder{prog: prog, defs: map[string]*method{}, left: map[string]string{}, results: map[string]typ{}, types: map[syntax.Node]typ{}, classByName: map[string]*class{}, news: map[*syntax.Call]*construction{}}
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

// builder holds the candidates: the methods still believed to compile,
// which every check may call directly, and what the check found out about
// them.
type builder struct {
	prog *syntax.Program
	// defs are the candidates by key: a top-level method's name, or
	// Class#name.
	defs map[string]*method
	left map[string]string
	// classes are the script's top-level classes, in order; classByName
	// finds one.
	classes     []*class
	classByName map[string]*class
	// results is the type of each candidate's value, unknown until a
	// check settled it.
	results map[string]typ
	// types is the type of every expression of the methods that compile,
	// as the last check saw it, for the generator.
	types map[syntax.Node]typ
	// news is what each Class.new the compiled methods make reaches.
	news map[*syntax.Call]*construction
	// cur is the method being checked, and ret the type of its returns.
	cur *method
	ret typ
}

// construction is a Class.new that compiles: the class, and how its fields
// are set, by the assignments of an initialize that does nothing else,
// from its parameters, or by keywords when the class has no initialize.
type construction struct {
	cls    *class
	init   *syntax.Def
	params []typ
	sets   []*syntax.FieldAssign
	// keys are the fields the keywords name, in the call's order.
	keys []int
}

// method is a candidate: its def, the class it belongs to, nil for a
// top-level method, and the type of each parameter.
type method struct {
	key    string
	def    *syntax.Def
	owner  *class
	params []typ
}

// goName is the Go name of the method's functions: its name, or
// Class__name for a class's method.
func (m *method) goName() string {
	return strings.ReplaceAll(m.key, "#", "__")
}

// class is a top-level class of the script: its fields in the order an
// instance holds them, the parent's first, and the methods it defines.
type class struct {
	name   string
	def    *syntax.ClassDef
	super  *class
	fields []*field
	index  map[string]int
	// t is the type of the class's instances.
	t typ
}

// field is one declared field: its name, the type of its value, and the
// class that declared it.
type field struct {
	name     string
	t        typ
	ref      string // the class a ref field names, resolved into t
	declared *class
	why      string // why a read of the field is not compiled, or ""
}

// collect picks the candidates: top-level defs, and the defs of top-level
// classes, whose name no other def in the program has, anywhere, with
// required positional parameters only. Another def of the name, in a class
// say, could be what a call reaches, and the compiled call would skip it.
func (b *builder) collect() {
	count := map[string]int{}
	walk(b.prog, func(n syntax.Node) {
		if d, ok := n.(*syntax.Def); ok {
			count[d.Name]++
		}
	})
	for _, n := range b.prog.Body {
		if c, ok := n.(*syntax.ClassDef); ok && c.Scope == nil {
			b.collectClass(c)
		}
	}
	for _, n := range b.prog.Body {
		switch n := n.(type) {
		case *syntax.Def:
			b.candidate(n, nil, count)
		case *syntax.ClassDef:
			c := b.classByName[n.Name]
			if c == nil || c.def != n {
				continue
			}
			for _, m := range n.Body {
				if d, ok := m.(*syntax.Def); ok {
					b.candidate(d, c, count)
				}
			}
		}
	}
	for _, m := range b.defs {
		m.params = b.paramTypes(m)
	}
}

// collectClass records a top-level class: its fields, after its parent's,
// when the parent is a top-level class of the script too. A class whose
// parent is anything else, or that is defined twice, has no layout the
// compiler knows, and its methods stay on the interpreter.
func (b *builder) collectClass(n *syntax.ClassDef) {
	if _, dup := b.classByName[n.Name]; dup {
		b.classByName[n.Name] = &class{name: n.Name}
		return
	}
	c := &class{name: n.Name, def: n, index: map[string]int{}}
	if n.Super != nil {
		sup, ok := n.Super.(*syntax.Const)
		if !ok || b.classByName[sup.Name] == nil || b.classByName[sup.Name].def == nil {
			b.classByName[n.Name] = &class{name: n.Name}
			return
		}
		c.super = b.classByName[sup.Name]
		for _, f := range c.super.fields {
			c.index[f.name] = len(c.fields)
			c.fields = append(c.fields, f)
		}
	}
	for _, m := range n.Body {
		d, ok := m.(*syntax.FieldDecl)
		if !ok {
			continue
		}
		for _, name := range d.Names {
			f := &field{name: name, declared: c}
			switch d.Type {
			case "int", "int32", "sint32", "sfixed32", "int64", "sint64", "sfixed64", "uint32", "fixed32", "uint64", "fixed64", "bignum":
				f.t = tInt
			case "float", "double", "float32":
				f.t = tFloat
			case "bool":
				f.t = tBool
			case "ref":
				if ref, ok := d.Ref.(*syntax.Const); ok {
					f.ref = ref.Name
				} else {
					f.why = "a ref field of a class not named plainly is not compiled yet"
				}
			default:
				f.why = fmt.Sprintf("a %s field is not compiled yet", d.Type)
			}
			c.index[name] = len(c.fields)
			c.fields = append(c.fields, f)
		}
	}
	c.t = tClass + typ(len(b.classes))
	b.classes = append(b.classes, c)
	b.classByName[n.Name] = c
}

// candidate takes a def as a candidate, or says why not.
func (b *builder) candidate(d *syntax.Def, owner *class, count map[string]int) {
	key := d.Name
	if owner != nil {
		key = owner.name + "#" + d.Name
	}
	switch {
	case d.Singleton:
		b.left[key] = "it is a singleton method"
	case reserved[d.Name]:
		b.left[key] = "the interpreter calls it itself"
	case count[d.Name] > 1:
		b.left[key] = "the program defines it more than once"
	case !goName(d.Name):
		b.left[key] = "its name is not a plain identifier"
	case !plainParams(d.Params):
		b.left[key] = "it takes optional, splat, keyword or block parameters"
	default:
		b.defs[key] = &method{key: key, def: d, owner: owner}
		b.results[key] = tUnknown
	}
}

// paramTypes finds the type of each parameter: an Integer, unless the body
// reads a field on it, which makes it an instance of the class that
// declares the field, or calls a candidate method of a class on it.
func (b *builder) paramTypes(m *method) []typ {
	types := make([]typ, len(m.def.Params.Positional))
	for i, p := range m.def.Params.Positional {
		types[i] = tInt
		var found *class
		walk(m.def, func(n syntax.Node) {
			call, ok := n.(*syntax.Call)
			if !ok || call.Recv == nil {
				return
			}
			v, ok := call.Recv.(*syntax.Var)
			if !ok || v.Name != p.Name {
				return
			}
			if c := b.declaring(call.Name); c != nil && found == nil {
				found = c
			}
		})
		if found != nil {
			types[i] = found.t
		}
	}
	return types
}

// declaring is the one class that declares a field of the name, or defines
// a candidate method of it; nil when none or several do.
func (b *builder) declaring(name string) *class {
	var found *class
	for _, c := range b.classes {
		declares := false
		if i, ok := c.index[name]; ok && c.fields[i].declared == c {
			declares = true
		}
		if _, ok := b.defs[c.name+"#"+name]; ok {
			declares = true
		}
		if declares {
			if found != nil {
				return nil
			}
			found = c
		}
	}
	return found
}

// reserved are the names the interpreter reaches past dispatch.
var reserved = map[string]bool{"initialize": true, "method_missing": true, "respond_to_missing": true, "to_s": true, "inspect": true}

// settle checks the candidates until nothing changes: one that does not
// check is dropped, and one that calls a dropped method no longer checks
// either; a result type found settles the calls of the method, which may
// settle their callers' types in turn. A method whose result stays unknown
// returns through itself alone and gets Integer, and a last round checks
// everything against the settled types, which the generator reads.
func (b *builder) settle() {
	for round := 0; ; round++ {
		changed := false
		names := make([]string, 0, len(b.defs))
		for name := range b.defs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t, err := b.check(b.defs[name])
			if err != nil {
				b.left[name] = err.Error()
				delete(b.defs, name)
				delete(b.results, name)
				changed = true
				continue
			}
			if t != b.results[name] {
				b.results[name] = t
				changed = true
			}
		}
		if changed {
			continue
		}
		settled := true
		for name, t := range b.results {
			if t == tUnknown {
				b.results[name] = tInt
				settled = false
			}
		}
		if settled {
			return
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
// break and next, which leave and have no value; unknown is the result of
// a method whose type the check has not settled yet, which joins with
// anything and is never the type of a value that compiles. From tClass on,
// a type is an instance of the script's class of that index.
type typ int

const (
	tInt typ = iota
	tFloat
	tBool
	tNever
	tUnknown
	tClass typ = 16
)

func (t typ) String() string {
	if t >= tClass {
		return "an instance"
	}
	return [...]string{"Integer", "Float", "true or false", "nothing", "not known yet"}[t]
}

// name names a type in a message, a class by its name.
func (b *builder) name(t typ) string {
	if t >= tClass {
		return "a " + b.classes[t-tClass].name
	}
	return t.String()
}

// classOf is the class of an instance type, nil for any other.
func (b *builder) classOf(t typ) *class {
	if t < tClass {
		return nil
	}
	return b.classes[t-tClass]
}

// numeric reports an Integer or a Float.
func numeric(t typ) bool { return t == tInt || t == tFloat }

// join is the type of an if whose branches have types a and b, or of a
// method's body and its returns.
func join(a, b typ) (typ, bool) {
	switch {
	case a == tNever:
		return b, true
	case b == tNever, a == b:
		return a, true
	case a == tUnknown:
		return b, true
	case b == tUnknown:
		return a, true
	}
	return 0, false
}

// assignable reports whether a value of type from may stand where to is
// wanted: the same type, or an instance of a class under to's.
func (b *builder) assignable(from, to typ) bool {
	if from == to || from == tNever || from == tUnknown {
		return true
	}
	for c := b.classOf(from); c != nil; c = c.super {
		if c.t == to {
			return true
		}
	}
	return false
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

// check types a method's body, and returns the type of its value: of the
// body's last expression and of its returns, which must agree.
func (b *builder) check(m *method) (typ, error) {
	sc := &scope{vars: map[string]typ{}}
	for i, p := range m.def.Params.Positional {
		sc.vars[p.Name] = m.params[i]
	}
	b.cur = m
	b.ret = tNever
	t, err := b.body(m.def.Body, sc, true)
	if err != nil {
		return 0, err
	}
	result, ok := join(t, b.ret)
	if !ok {
		return 0, fmt.Errorf("its value is %s and it returns %s", b.name(t), b.name(b.ret))
	}
	if !numeric(result) && result != tUnknown && result < tClass {
		return 0, fmt.Errorf("its result is %s, not a number or an instance", b.name(result))
	}
	return result, nil
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

// expr checks one expression and records its type for the generator;
// value says whether its value is used.
func (b *builder) expr(n syntax.Node, sc *scope, value bool) (typ, error) {
	t, err := b.typeExpr(n, sc, value)
	if err == nil {
		b.types[n] = t
	}
	return t, err
}

func (b *builder) typeExpr(n syntax.Node, sc *scope, value bool) (typ, error) {
	switch n := n.(type) {
	case *syntax.Int:
		return tInt, nil
	case *syntax.Float:
		return tFloat, nil
	case *syntax.Bool:
		return tBool, nil
	case *syntax.Self:
		if b.cur.owner == nil {
			return 0, fmt.Errorf("line %d: self at the top level is not compiled", n.Line())
		}
		return b.cur.owner.t, nil
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
		case ok && old != t && old != tUnknown && t != tUnknown:
			return 0, fmt.Errorf("line %d: %s holds %s and %s", n.Line(), n.Name, b.name(old), b.name(t))
		case !ok && sc.nested > 0:
			return 0, fmt.Errorf("line %d: %s is first set inside an if or a loop", n.Line(), n.Name)
		case ok && t == tUnknown:
			return old, nil
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
		t, err := b.expr(n.Value, sc, true)
		if err != nil {
			return 0, err
		}
		if !numeric(t) && t != tUnknown && t < tClass {
			return 0, fmt.Errorf("line %d: the value of return is %s, not a number or an instance", n.Line(), b.name(t))
		}
		ret, ok := join(b.ret, t)
		if !ok {
			return 0, fmt.Errorf("line %d: it returns %s and %s", n.Line(), b.name(b.ret), b.name(t))
		}
		b.ret = ret
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
			return 0, fmt.Errorf("line %d: the branches give %s and %s", n.Line(), b.name(then), b.name(other))
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

// want checks an expression that must have one type; a never and an
// unknown pass, since they are no value of another type.
func (b *builder) want(n syntax.Node, sc *scope, want typ, what string) (typ, error) {
	t, err := b.expr(n, sc, true)
	if err != nil {
		return 0, err
	}
	if !b.assignable(t, want) {
		return 0, fmt.Errorf("line %d: %s is %s, not %s", n.Line(), what, b.name(t), b.name(want))
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

// arith and compare are the operators of Integer and Float the compiler
// knows, methods a script cannot redefine; conversions are their to_f and
// to_i.
var arith = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true}
var compare = map[string]bool{"<": true, "<=": true, ">": true, ">=": true, "==": true, "!=": true}
var conversions = map[string]typ{"to_f": tFloat, "to_i": tInt}

func isConversion(name string) bool {
	_, ok := conversions[name]
	return ok
}

// loops are the methods that call a block once per Integer, which compile
// to a Go loop with the block's body inside: times, upto and downto on an
// Integer, each on a Range of Integers.
var loops = map[string]bool{"times": true, "upto": true, "downto": true, "each": true}

// target is what a call reaches: a field of a class, read from an
// instance, or a candidate method, called directly.
type target struct {
	field  *field
	index  int
	method *method
}

// resolve finds what a call of name on an instance of class c reaches: a
// field of c, or a candidate method of c or of a class above it. nil when
// neither.
func (b *builder) resolve(c *class, name string) *target {
	if i, ok := c.index[name]; ok {
		return &target{field: c.fields[i], index: i}
	}
	for k := c; k != nil; k = k.super {
		if m, ok := b.defs[k.name+"#"+name]; ok {
			return &target{method: m}
		}
	}
	return nil
}

// bare finds what a call without a receiver reaches from a method of
// owner, nil for a top-level one: a field or a method of the class first,
// then a top-level method.
func (b *builder) bare(owner *class, name string) *target {
	if owner != nil {
		if t := b.resolve(owner, name); t != nil {
			return t
		}
	}
	if m, ok := b.defs[name]; ok && m.owner == nil {
		return &target{method: m}
	}
	return nil
}

func (b *builder) call(n *syntax.Call, sc *scope, value bool) (typ, error) {
	if c, ok := n.Recv.(*syntax.Const); ok && n.Name == "new" && n.BlockArg == nil && !n.Safe {
		return b.construct(n, c.Name, sc)
	}
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
		t := b.bare(b.cur.owner, n.Name)
		if t == nil {
			why := "it is not a method of the script"
			if _, isLeft := b.left[n.Name]; isLeft {
				why = "it is left to the interpreter"
			} else if b.cur.owner != nil {
				if _, isLeft := b.left[b.cur.owner.name+"#"+n.Name]; isLeft {
					why = "it is left to the interpreter"
				}
			}
			return 0, fmt.Errorf("line %d: it calls %s, and %s", n.Line(), n.Name, why)
		}
		return b.reach(n, t, sc)
	}
	recv, err := b.expr(n.Recv, sc, true)
	if err != nil {
		return 0, err
	}
	if c := b.classOf(recv); c != nil {
		t := b.resolve(c, n.Name)
		if t == nil {
			return 0, fmt.Errorf("line %d: it calls %s on %s, which is not a field or a compiled method of it", n.Line(), n.Name, b.name(recv))
		}
		return b.reach(n, t, sc)
	}
	switch {
	case n.Name == "-@" && len(n.Args) == 0 && (numeric(recv) || recv == tUnknown):
		return recv, nil
	case len(n.Args) == 0 && isConversion(n.Name) && (numeric(recv) || recv == tUnknown):
		return conversions[n.Name], nil
	case len(n.Args) == 1 && (arith[n.Name] || compare[n.Name]):
		arg, err := b.expr(n.Args[0], sc, true)
		if err != nil {
			return 0, err
		}
		numbers := (numeric(recv) || recv == tUnknown) && (numeric(arg) || arg == tUnknown)
		switch {
		case numbers && compare[n.Name]:
			return tBool, nil
		case numbers && (recv == tFloat || arg == tFloat):
			return tFloat, nil
		case numbers && (recv == tUnknown || arg == tUnknown):
			return tUnknown, nil
		case numbers:
			return tInt, nil
		case recv == tBool && arg == tBool && (n.Name == "==" || n.Name == "!="):
			return tBool, nil
		}
		return 0, fmt.Errorf("line %d: %s %s %s is not compiled", n.Line(), b.name(recv), n.Name, b.name(arg))
	}
	return 0, fmt.Errorf("line %d: it calls %s on %s", n.Line(), n.Name, b.name(recv))
}

// reach checks a call that reaches a field or a candidate method: a field
// takes no arguments and has its declared type; a method takes what its
// parameters are and has its settled result.
func (b *builder) reach(n *syntax.Call, t *target, sc *scope) (typ, error) {
	if t.field != nil {
		if len(n.Args) != 0 {
			return 0, fmt.Errorf("line %d: the field %s takes no arguments", n.Line(), n.Name)
		}
		if t.field.why != "" {
			return 0, fmt.Errorf("line %d: %s", n.Line(), t.field.why)
		}
		if t.field.ref != "" {
			c := b.classByName[t.field.ref]
			if c == nil || c.def == nil {
				return 0, fmt.Errorf("line %d: the field %s refers to %s, which is not a top-level class of the script", n.Line(), n.Name, t.field.ref)
			}
			return c.t, nil
		}
		return t.field.t, nil
	}
	m := t.method
	if len(n.Args) != len(m.params) {
		return 0, fmt.Errorf("line %d: it calls %s with %d arguments, not %d", n.Line(), n.Name, len(n.Args), len(m.params))
	}
	for i, a := range n.Args {
		if _, err := b.want(a, sc, m.params[i], "an argument of "+n.Name); err != nil {
			return 0, err
		}
	}
	return b.results[m.key], nil
}

// construct checks Class.new of a top-level class of the script: with an
// initialize, one whose body sets fields from its parameters and does
// nothing else, called with its arguments; without one, keywords naming
// fields. A field takes an Integer where it holds a Float, as the
// interpreter converts it.
func (b *builder) construct(n *syntax.Call, name string, sc *scope) (typ, error) {
	cls := b.classByName[name]
	if cls == nil || cls.def == nil {
		return 0, fmt.Errorf("line %d: it constructs %s, which is not a top-level class of the script", n.Line(), name)
	}
	if n.Block != nil {
		return 0, fmt.Errorf("line %d: %s.new with a block is not compiled yet", n.Line(), name)
	}
	k := &construction{cls: cls}
	init := b.initializer(cls)
	if init == nil {
		if len(n.Args) > 0 {
			return 0, fmt.Errorf("line %d: %s.new takes its fields as keywords", n.Line(), name)
		}
		for _, kw := range n.KwArgs {
			sym, ok := kw.Key.(*syntax.Sym)
			if !ok {
				return 0, fmt.Errorf("line %d: %s.new names its fields by symbols", n.Line(), name)
			}
			i, ok := cls.index[sym.Name]
			if !ok {
				return 0, fmt.Errorf("line %d: %s.new: %s has no field %s", n.Line(), name, name, sym.Name)
			}
			if err := b.setField(kw.Value, cls.fields[i], sc, n.Line()); err != nil {
				return 0, err
			}
			k.keys = append(k.keys, i)
		}
		b.news[n] = k
		return cls.t, nil
	}
	if len(n.KwArgs) > 0 {
		return 0, fmt.Errorf("line %d: %s.new with keywords runs initialize, which takes positional arguments", n.Line(), name)
	}
	if !plainParams(init.Params) {
		return 0, fmt.Errorf("line %d: %s.new runs an initialize with optional, splat, keyword or block parameters", n.Line(), name)
	}
	if len(n.Args) != len(init.Params.Positional) {
		return 0, fmt.Errorf("line %d: it calls %s.new with %d arguments, not %d", n.Line(), name, len(n.Args), len(init.Params.Positional))
	}
	k.init = init
	for _, m := range init.Body {
		set, ok := m.(*syntax.FieldAssign)
		if !ok {
			return 0, fmt.Errorf("line %d: %s.new runs an initialize that does more than set fields: %s", n.Line(), name, describe(m))
		}
		if _, ok := cls.index[set.Name]; !ok {
			return 0, fmt.Errorf("line %d: %s.new: initialize sets %s, which %s has no field of", n.Line(), name, set.Name, name)
		}
		k.sets = append(k.sets, set)
	}
	k.params = b.initParams(init, cls)
	for i, a := range n.Args {
		if err := b.argument(a, k.params[i], sc, n.Line(), name+".new"); err != nil {
			return 0, err
		}
	}
	inner := &scope{vars: map[string]typ{}}
	for i, p := range init.Params.Positional {
		inner.vars[p.Name] = k.params[i]
	}
	for _, set := range k.sets {
		if err := b.setField(set.Value, cls.fields[cls.index[set.Name]], inner, set.Line()); err != nil {
			return 0, err
		}
	}
	b.news[n] = k
	return cls.t, nil
}

// initializer is the initialize an instance of c runs: its own, or the
// nearest above it; nil when there is none.
func (b *builder) initializer(c *class) *syntax.Def {
	for k := c; k != nil; k = k.super {
		for _, m := range k.def.Body {
			if d, ok := m.(*syntax.Def); ok && d.Name == "initialize" && !d.Singleton {
				return d
			}
		}
	}
	return nil
}

// initParams types initialize's parameters: one that the body assigns to
// fields and uses nowhere else has the type of the field, so that a Float
// field takes a Float; any other is an Integer.
func (b *builder) initParams(init *syntax.Def, c *class) []typ {
	types := make([]typ, len(init.Params.Positional))
	for i, p := range init.Params.Positional {
		types[i] = tInt
		uses, direct := 0, 0
		var t typ = tInt
		fits := true
		walk(init, func(n syntax.Node) {
			if v, ok := n.(*syntax.Var); ok && v.Name == p.Name {
				uses++
			}
		})
		for _, m := range init.Body {
			set, ok := m.(*syntax.FieldAssign)
			if !ok {
				continue
			}
			if v, ok := set.Value.(*syntax.Var); ok && v.Name == p.Name {
				ft, err := b.fieldType(c.fields[c.index[set.Name]])
				if err != nil || direct > 0 && ft != t {
					fits = false
				}
				t = ft
				direct++
			}
		}
		if fits && direct > 0 && direct == uses {
			types[i] = t
		}
	}
	return types
}

// fieldType is the type a field holds: of a ref field, the instances of
// the class it names.
func (b *builder) fieldType(f *field) (typ, error) {
	if f.why != "" {
		return 0, fmt.Errorf("%s", f.why)
	}
	if f.ref != "" {
		c := b.classByName[f.ref]
		if c == nil || c.def == nil {
			return 0, fmt.Errorf("the field %s refers to %s, which is not a top-level class of the script", f.name, f.ref)
		}
		return c.t, nil
	}
	return f.t, nil
}

// setField checks the value a construction gives a field: of the field's
// type, or an Integer for a Float field, which the interpreter converts.
func (b *builder) setField(value syntax.Node, f *field, sc *scope, line int) error {
	ft, err := b.fieldType(f)
	if err != nil {
		return fmt.Errorf("line %d: %s", line, err)
	}
	return b.argument(value, ft, sc, line, "the field "+f.name)
}

// argument checks a value given where a type is wanted, in a construction:
// assignable, or an Integer for a Float.
func (b *builder) argument(value syntax.Node, want typ, sc *scope, line int, what string) error {
	t, err := b.expr(value, sc, true)
	if err != nil {
		return err
	}
	if !b.assignable(t, want) && !(t == tInt && want == tFloat) {
		return fmt.Errorf("line %d: %s takes %s, not %s", line, what, b.name(want), b.name(t))
	}
	return nil
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
			return 0, fmt.Errorf("line %d: it calls each on %s", n.Line(), b.name(t))
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
	"FieldAssign": "a field assignment", "Async": "async", "Await": "await",
}

// emit writes the Go source.
func (b *builder) emit(file, src, pkg string, names []string) ([]byte, error) {
	g := &gen{b: b, types: b.types, results: b.results, helpers: map[string]bool{}, consts: map[int64]string{}}
	var fns bytes.Buffer
	for _, name := range names {
		fns.WriteString(g.function(b.defs[name], false))
		fns.WriteString(g.function(b.defs[name], true))
		fns.WriteString(g.entry(b.defs[name]))
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by cameo build from %s; DO NOT EDIT.\n\n", baseName(file))
	fmt.Fprintf(&out, "package %s\n\n", pkg)
	out.WriteString("import (\n")
	if pkg == "main" {
		out.WriteString("\t\"fmt\"\n")
	}
	if g.helpers["fmod"] || g.helpers["bigToInt"] {
		out.WriteString("\t\"math\"\n")
	}
	if len(names) > 0 {
		out.WriteString("\t\"math/big\"\n")
	}
	if pkg == "main" {
		out.WriteString("\t\"os\"\n\t\"runtime/debug\"\n")
	}
	out.WriteString("\t\"sync/atomic\"\n\n")
	if pkg == "main" {
		out.WriteString("\t\"github.com/cameo-lang/cameo/src/cameo\"\n")
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
		fmt.Fprintf(&out, "\t\t%s: call_%s,\n", strconv.Quote(name), b.defs[name].goName())
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
		if status, ok := cameo.ExitStatus(err); ok {
			os.Exit(status)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

`

// stateType is what every compiled call carries: the run it belongs to,
// its stop flag, which every call and loop pass reads, and the classes it
// constructs, found once.
const stateType = `// state is one call from the interpreter into compiled code: the run it
// belongs to, the flag set when the run is stopped, and the classes the
// call constructed instances of.
type state struct {
	ctx     object.Context
	stop    *atomic.Bool
	classes map[string]*object.Class
}

// class is the class the script's top-level constant of that name holds,
// found once per call into compiled code; nil when there is none, which
// eval.Construct leaves to the interpreter.
func (s *state) class(name string) *object.Class {
	c, ok := s.classes[name]
	if !ok {
		if s.classes == nil {
			s.classes = map[string]*object.Class{}
		}
		c = eval.ClassNamed(s.ctx, name)
		s.classes[name] = c
	}
	return c
}

`

// entry is the method the interpreter calls: it takes the arguments as
// Values, runs on int64 while the Integers fit, on *big.Int once one does
// not, and leaves any call it cannot run to the interpreter.
func (g *gen) entry(m *method) string {
	var b strings.Builder
	d := m.def
	name := m.goName()
	n := len(d.Params.Positional)
	asInt, asBig := "object.Int(r)", "object.BigInt(r)"
	switch result := g.results[m.key]; {
	case result == tFloat:
		asInt, asBig = "object.Float(r)", "object.Float(r)"
	case result >= tClass:
		asInt, asBig = "object.InstanceValue(r)", "object.InstanceValue(r)"
	}
	fmt.Fprintf(&b, "// call_%s is %s as a method: it runs with the arguments it takes and no\n// block, on int64 while the Integers fit and on *big.Int once one does not,\n// and leaves every other call to the interpreter.\n", name, m.key)
	fmt.Fprintf(&b, "func call_%s(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {\n", name)
	fmt.Fprintf(&b, "\tif len(args) != %d || kw != nil || !(block.IsZero() || block.IsNil()) {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n", n)
	b.WriteString("\ts := &state{ctx: ctx, stop: eval.StopFlag(ctx)}\n")
	var small, large, fits []string
	if m.owner != nil {
		b.WriteString("\tself_, ok := self.AsInstance()\n\tif !ok {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n")
		small, large = []string{"self_"}, []string{"self_"}
	} else {
		b.WriteString("\t_ = self\n")
	}
	for i := range d.Params.Positional {
		if c := g.b.classOf(m.params[i]); c != nil {
			g.helpers["classNamed"] = true
			fmt.Fprintf(&b, "\ta%d, ok%d := args[%d].AsInstance()\n\tif !ok%d || !classNamed(a%d.Cls, %s) {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n", i, i, i, i, i, strconv.Quote(c.name))
			small = append(small, fmt.Sprintf("a%d", i))
			large = append(large, fmt.Sprintf("a%d", i))
			continue
		}
		fmt.Fprintf(&b, "\ta%d, ok%d := args[%d].AsInt()\n", i, i, i)
		small = append(small, fmt.Sprintf("a%d", i))
		large = append(large, fmt.Sprintf("b%d", i))
		fits = append(fits, fmt.Sprintf("ok%d", i))
	}
	if n == 0 {
		b.WriteString("\t_ = args\n")
	}
	if len(fits) == 0 {
		fits = append(fits, "true")
	}
	fmt.Fprintf(&b, "\tif %s {\n", strings.Join(fits, " && "))
	fmt.Fprintf(&b, "\t\tr, err := fn_%s(s%s)\n", name, prefixed(small))
	fmt.Fprintf(&b, "\t\tif err != eval.ErrDeopt {\n\t\t\tif err != nil {\n\t\t\t\treturn object.Value{}, err\n\t\t\t}\n\t\t\treturn %s, nil\n\t\t}\n\t}\n", asInt)
	for i := range d.Params.Positional {
		if g.b.classOf(m.params[i]) != nil {
			continue
		}
		fmt.Fprintf(&b, "\tb%d, ok := args[%d].AsBigInt()\n\tif !ok {\n\t\treturn object.Value{}, eval.ErrDeopt\n\t}\n", i, i)
	}
	fmt.Fprintf(&b, "\tr, err := big_%s(s%s)\n", name, prefixed(large))
	fmt.Fprintf(&b, "\tif err != nil {\n\t\treturn object.Value{}, err\n\t}\n\treturn %s, nil\n}\n\n", asBig)
	return b.String()
}

func prefixed(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return ", " + strings.Join(args, ", ")
}
