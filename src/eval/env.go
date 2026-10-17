package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// Env is a lexical scope of local variables. A block's scope has the scope
// it was written in as parent and sees its variables; a method body starts
// a scope with no parent.
type Env struct {
	vars   map[string]object.Value
	parent *Env
}

// NewEnv makes a scope inside parent (nil for a method body).
func NewEnv(parent *Env) *Env {
	return &Env{vars: map[string]object.Value{}, parent: parent}
}

// Get reads a variable, searching outwards, and false when it is unbound.
func (e *Env) Get(name string) (object.Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// Set rebinds a variable where it is bound, or binds it in this scope when
// it is new: a block's new variable is the block's own.
func (e *Env) Set(name string, v object.Value) {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return
		}
	}
	e.vars[name] = v
}

// Define binds a variable in this scope, shadowing an outer one: what a
// parameter does.
func (e *Env) Define(name string, v object.Value) { e.vars[name] = v }

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
	// method on its argument, and Block is nil.
	Sym string
}

func (*Proc) Class() *object.Class { return object.ProcClass }

// Arity is the number of required positional parameters, negative when the
// block takes a splat or optional parameters, as Proc#arity answers.
func (p *Proc) Arity() int {
	if p.Block == nil {
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
