package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// The forms below belong to later milestones (fields and copies in 4, async
// in 6). Until then they fail with the milestone that brings them.

func (in *Interp) evalFieldDecl(n *syntax.FieldDecl, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "%s :%s: typed fields come with milestone 4", n.Type, n.Names[0])
}

func (in *Interp) evalReserved(n *syntax.Reserved, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "reserved comes with milestone 4")
}

func (in *Interp) evalInclude(n *syntax.Include, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "include and extend come with milestone 4")
}

func (in *Interp) evalWith(n *syntax.With, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "assignment into a copy comes with milestone 4")
}

func (in *Interp) evalAsync(n *syntax.Async, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "async comes with milestone 6")
}

func (in *Interp) evalAwait(n *syntax.Await, env *Env, fr *Frame) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, n.Line(), "await comes with milestone 6")
}

func (in *Interp) evalAwaitAll(fr *Frame, line int) (object.Value, error) {
	return nil, in.raise(object.NotImplementedErr, line, "await :all comes with milestone 6")
}

// checkField checks a value against a field's declared type; fields come
// with milestone 4, so until then every value passes.
func (in *Interp) checkField(f *object.Field, v object.Value, line int) (object.Value, error) {
	return v, nil
}

// inheritFields copies a superclass's fields into a new class.
func (in *Interp) inheritFields(cls, super *object.Class) error {
	for _, f := range super.Fields {
		cls.FieldIndex[f.Name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
	return nil
}
