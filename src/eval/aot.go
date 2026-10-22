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

// Stopped reports whether the run that ctx belongs to was stopped. A
// compiled method checks it now and then, as the interpreter does at
// every call, and returns ErrStopped.
func Stopped(ctx object.Context) bool {
	in, ok := ctx.(*Interp)
	return ok && in.stop.flag.Load()
}
