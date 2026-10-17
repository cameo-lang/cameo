package eval

import (
	"unsafe"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// Env is a lexical scope of local variables. A block's scope has the scope
// it was written in as parent and sees its variables; a method body starts
// a scope with no parent.
//
// The first few variables live in the scope itself, so a scope of a method
// with a handful of locals costs no allocation beyond its own; more spill
// into a map.
type Env struct {
	names  [envInline]string
	values [envInline]object.Value
	n      int
	more   map[string]object.Value
	parent *Env
	// processes are the async processes started in this scope, which
	// `await :all` collects.
	processes []*Process
}

// envInline is how many variables a scope holds without a map.
const envInline = 4

// NewEnv makes a scope inside parent (nil for a method body).
func NewEnv(parent *Env) *Env {
	return &Env{parent: parent}
}

// lookup finds name in this scope alone.
func (e *Env) lookup(name string) (object.Value, bool) {
	for i := 0; i < e.n; i++ {
		if e.names[i] == name {
			return e.values[i], true
		}
	}
	if e.more != nil {
		v, ok := e.more[name]
		return v, ok
	}
	return object.Value{}, false
}

// bind binds name in this scope alone, rebinding it when it is bound.
func (e *Env) bind(name string, v object.Value) {
	for i := 0; i < e.n; i++ {
		if e.names[i] == name {
			e.values[i] = v
			return
		}
	}
	if e.n < envInline {
		e.names[e.n] = name
		e.values[e.n] = v
		e.n++
		return
	}
	if e.more == nil {
		e.more = map[string]object.Value{}
	}
	e.more[name] = v
}

// Get reads a variable, searching outwards, and false when it is unbound.
func (e *Env) Get(name string) (object.Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.lookup(name); ok {
			return v, true
		}
	}
	return object.Value{}, false
}

// Set rebinds a variable where it is bound, or binds it in this scope when
// it is new: a block's new variable is the block's own.
func (e *Env) Set(name string, v object.Value) {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.lookup(name); ok {
			s.bind(name, v)
			return
		}
	}
	e.bind(name, v)
}

// Define binds a variable in this scope, shadowing an outer one: what a
// parameter does.
func (e *Env) Define(name string, v object.Value) { e.bind(name, v) }

// activation is a method's frame and its scope in one allocation.
type activation struct {
	frame Frame
	env   Env
}

// Frame is one method activation: who self is, which method runs, with what
// arguments and block, and the lexical scopes constants are looked up in.
// A block runs in the frame it was written in.
type Frame struct {
	Self   object.Value
	Method *object.Method
	// Owner is the class that defines Method, where super continues from.
	Owner *object.Class
	Args  []object.Value
	Kw    *object.Map
	Block object.Value // the block passed to the method, a *Proc, or nil
	// Lexical is the chain of classes and modules enclosing the code,
	// innermost last; nil at the top level.
	Lexical []*object.Class
	// DefTarget is the class or module a def inside this frame defines on:
	// the class whose body is being evaluated, or Object at the top level.
	DefTarget *object.Class
	// Building is the instance initialize is filling, while it runs.
	Building *object.Instance
	// Lambda marks the frame of a lambda call, where return stops.
	Lambda bool
}

// Proc is a block or lambda: a body closed over the scope and frame it was
// written in. Strict is a lambda: arguments are counted, and return returns
// from the lambda.
type Proc struct {
	Block  *syntax.Block
	Env    *Env
	Frame  *Frame
	Strict bool
	// Sym is set for a block made from a symbol (&:name): it calls that
	// method on its argument, and Block is nil. With Recv set it calls the
	// method on Recv with the block's arguments instead, as the proc of a
	// hash (&hash) calls [].
	Sym  string
	Recv object.Value
	// Go is set for a block a builtin makes: it runs the function with the
	// block's arguments, and Block is nil.
	Go func(args []object.Value) (object.Value, error)
}

// procValue wraps a proc as a value; proc and asProc read it back.
func procValue(p *Proc) object.Value { return object.Opaque(object.KindProc, unsafe.Pointer(p)) }

func proc(v object.Value) *Proc {
	p, ok := asProc(v)
	if !ok {
		panic("eval: " + v.Kind().String() + " read as proc")
	}
	return p
}

func asProc(v object.Value) (*Proc, bool) {
	if v.Kind() != object.KindProc {
		return nil, false
	}
	return (*Proc)(v.Opaque()), true
}

// Arity is the number of required positional parameters, negative when the
// block takes a splat or optional parameters, as Proc#arity reports.
func (p *Proc) Arity() int {
	if p.Block == nil {
		if p.Go != nil {
			return -1
		}
		if !p.Recv.IsZero() {
			return 1
		}
		return -2
	}
	required, variable := 0, false
	for _, param := range p.Block.Params.Positional {
		switch {
		case param.Splat, param.Default != nil:
			variable = true
		default:
			required++
		}
	}
	if variable || (!p.Strict && len(p.Block.Params.Keywords) > 0) {
		return -(required + 1)
	}
	return required
}
