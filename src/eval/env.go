package eval

import (
	"sort"
	"unsafe"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Env is a lexical scope of local variables. A block's scope has the scope
// it was written in as parent and sees its variables; a method body starts
// a scope with no parent.
//
// A block captures values, not variables: when a block is written, the
// variables it sees are copied into a captured scope (snapshot), and that
// is the scope its calls run in. A rebinding in the enclosing scope after
// that is invisible to the block, and a rebinding inside the block lands
// in the call's own scope, so it is gone when the call returns. Nothing
// observes a variable from outside its scope, which is what an immutable
// value needs.
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
	// captured marks a scope a block captured: Set never rebinds in it.
	captured bool
	// scope is, for a captured scope, the method scope it was copied
	// from, where the promises of a block's async calls belong.
	scope *Env
	// promises are the ones async started in this scope, which `await
	// :all` collects.
	promises []*Promise
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

// each calls f with every variable of this scope alone, in binding order
// for the inline ones and by name for the rest.
func (e *Env) each(f func(name string, v object.Value)) {
	for i := 0; i < e.n; i++ {
		f(e.names[i], e.values[i])
	}
	names := make([]string, 0, len(e.more))
	for name := range e.more {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f(name, e.more[name])
	}
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
// it is new or captured: a block's new variable is the block's own, and so
// is its rebinding of a captured one.
func (e *Env) Set(name string, v object.Value) {
	for s := e; s != nil && !s.captured; s = s.parent {
		if _, ok := s.lookup(name); ok {
			s.bind(name, v)
			return
		}
	}
	e.bind(name, v)
}

// blockEnv is the scope a block closes over: a snapshot of env, or env
// itself for the body of a for loop.
func blockEnv(b *syntax.Block, env *Env) *Env {
	if b.Shared {
		return env
	}
	return env.snapshot()
}

// snapshot copies every variable the scope sees into a captured scope, the
// values they have now, an inner binding shadowing an outer one. A block
// closes over the snapshot, not the scope.
func (e *Env) snapshot() *Env {
	s := &Env{captured: true, scope: methodScope(e)}
	e.copyInto(s)
	return s
}

// copyInto binds the variables of this scope and its parents in s, the
// outermost first so that an inner binding wins.
func (e *Env) copyInto(s *Env) {
	if e.parent != nil {
		e.parent.copyInto(s)
	}
	for i := 0; i < e.n; i++ {
		s.bind(e.names[i], e.values[i])
	}
	for name, v := range e.more {
		s.bind(name, v)
	}
}

// methodScope is the scope a promise belongs to: the method body's, or the
// top level's, that the current block scope sits in. A block is part of
// the method that runs it, so a method that fans out in a loop and then
// waits sees every promise.
func methodScope(env *Env) *Env {
	for env.parent != nil {
		env = env.parent
	}
	if env.scope != nil {
		return env.scope
	}
	return env
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
	// Reopened is the class DefTarget is a copy of while a body reopens
	// it, whose fields the body may declare again with the same type.
	Reopened *object.Class
	// Building is the instance initialize is filling, while it runs, and
	// BuildingExc the exception.
	Building    *object.Instance
	BuildingExc *object.Exception
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
