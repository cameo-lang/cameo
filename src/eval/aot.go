package eval

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// ErrDeopt is what a compiled method returns when it cannot finish a call
// in Go, an Integer that outgrew 64 bits for one: the interpreter then runs
// the method's own body from the start. A compiled method is pure, so
// starting over repeats nothing anyone could see.
var ErrDeopt = errors.New("cameo: the compiled method leaves this call to the interpreter")

// Compiled is the Go implementation that `cameo build` made of the
// top-level methods of one program, by method name.
type Compiled map[string]object.NativeFn

// compiledPrograms holds the registered implementations by the digest of
// the program they were compiled from. Keying on the whole program keeps
// them from any other script: a method compiled to call fib directly is
// right only where no other definition of fib exists.
var compiledPrograms sync.Map // digest -> Compiled

// anyCompiled is set by the first RegisterCompiled, so that a run in a
// process without compiled code never computes a digest.
var anyCompiled atomic.Bool

// RegisterCompiled makes methods the Go implementation of the top-level
// methods of the program whose digest is given, syntax.Digest of the parsed
// program. The generated code of `cameo build` calls it from init.
func RegisterCompiled(digest string, methods Compiled) {
	compiledPrograms.Store(digest, methods)
	anyCompiled.Store(true)
}

// CompiledFor returns the compiled methods registered for prog, nil when
// there are none.
func CompiledFor(prog *syntax.Program) Compiled {
	if !anyCompiled.Load() {
		return nil
	}
	if c, ok := compiledPrograms.Load(syntax.Digest(prog)); ok {
		return c.(Compiled)
	}
	return nil
}

// StopFlag returns the flag that is set when the run ctx belongs to is
// stopped. A compiled method reads it at every call and loop pass, as the
// interpreter does, and returns ErrStopped once it is set. A ctx that is
// not a run gets a flag that is never set.
func StopFlag(ctx object.Context) *atomic.Bool {
	if in, ok := ctx.(*Interp); ok {
		return &in.stop.flag
	}
	return new(atomic.Bool)
}

// ClassNamed returns the class a top-level constant of the script names,
// nil when there is no such constant or it is not a class. Compiled code
// finds the class it constructs this way, once per call into it.
func ClassNamed(ctx object.Context, name string) *object.Class {
	in, ok := ctx.(*Interp)
	if !ok {
		return nil
	}
	v, ok := in.consts[name]
	if !ok {
		return nil
	}
	c, _ := v.AsClass()
	return c
}

// Construct makes an instance of cls as Class.new does: every field at its
// default, then each field that values gives, one entry per field of the
// class with the zero Value for one left at its default, checked and
// converted as `self.name = value` in initialize checks it. It returns
// ErrDeopt when cls is nil or a value does not fit its field, so that the
// interpreter builds the instance and raises what it raises.
func Construct(ctx object.Context, cls *object.Class, values []object.Value) (*object.Instance, error) {
	in, ok := ctx.(*Interp)
	if !ok || cls == nil {
		return nil, ErrDeopt
	}
	inst := &object.Instance{Cls: cls, Fields: make([]object.Value, len(cls.Fields))}
	for i, f := range cls.Fields {
		inst.Fields[i] = f.Default
		if f.Default.IsZero() {
			inst.Fields[i] = emptyFor(f.Type)
		}
		if i >= len(values) || values[i].IsZero() {
			continue
		}
		v, ok := in.coerce(f.Type, values[i])
		if !ok {
			return nil, ErrDeopt
		}
		inst.Fields[i] = v
	}
	return inst, nil
}
