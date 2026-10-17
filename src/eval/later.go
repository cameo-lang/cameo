package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// async belongs to milestone 6. Until then its forms fail with the
// milestone that brings them.

func (in *Interp) evalAsync(n *syntax.Async, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "async comes with milestone 6")
}

func (in *Interp) evalAwait(n *syntax.Await, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "await comes with milestone 6")
}

func (in *Interp) evalAwaitAll(fr *Frame, line int) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, line, "await :all comes with milestone 6")
}
