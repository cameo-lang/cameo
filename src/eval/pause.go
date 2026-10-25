package eval

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A run pauses between two top-level statements, never inside one: the
// evaluator keeps a running statement's state on the Go stack, which no
// dump can hold. What a dump holds, and how a resumed run gets its classes
// back, is in doc/design/pause.md.

// ErrPaused ends a run that Pause asked to pause. It is not an exception,
// so no rescue catches it.
var ErrPaused = errors.New("the script paused")

// Pause asks the run to pause before its next top-level statement: Run
// then returns ErrPaused, and Dump captures the run. A statement that never
// ends is never paused; Stop ends it.
func (in *Interp) Pause() { in.pause.Store(true) }

// Dump captures a paused run: the script, where it paused, its top-level
// variables, the constants it assigned, and its promises. It waits for the
// calls async started, so their results travel settled. Source is the
// script's text, which the dump carries for the run that resumes it.
func (in *Interp) Dump(source string) (*protogen.Dump, error) {
	if !in.paused {
		return nil, errors.New("the script is not paused")
	}
	d := &protogen.Dump{File: in.file, Source: source, Next: in.next, Line: in.nextLine}
	enc := in.Encoder()
	in.top.each(func(name string, v object.Value) {
		d.Variables = append(d.Variables, in.binding(enc, name, v))
	})
	for _, name := range sortedKeys(in.consts) {
		v := in.consts[name]
		// A class the script defined under its own name comes back from
		// its definition.
		if cls, ok := v.AsClass(); ok && cls.Name == name {
			continue
		}
		d.Constants = append(d.Constants, in.binding(enc, name, v))
	}
	promises, err := in.settledPromises(enc)
	if err != nil {
		return nil, err
	}
	d.Promises = promises
	for lib := range in.loaded {
		d.Loaded = append(d.Loaded, lib)
	}
	sort.Strings(d.Loaded)
	if d.Rand, err = in.rng.MarshalBinary(); err != nil {
		return nil, err
	}
	return d, nil
}

// binding is the dump's form of one variable or constant.
func (in *Interp) binding(enc *protogen.Encoder, name string, v object.Value) protogen.Binding {
	b := protogen.Binding{Name: name}
	if cls, ok := v.AsClass(); ok {
		b.Class = cls.QualifiedName()
		return b
	}
	data, err := enc.Encode(v)
	if err != nil {
		b.Missing = err.Error()
		return b
	}
	b.Value = data
	return b
}

// settledPromises waits for every call async started and returns the
// promises of the run, settled. A promise the host or the script made by
// hand and has not settled is left out.
func (in *Interp) settledPromises(enc *protogen.Encoder) ([]protogen.PromiseState, error) {
	t := in.promises
	var out []protogen.PromiseState
	for id := int64(1); id <= t.next.Load(); id++ {
		p := t.get(id)
		if p == nil {
			continue
		}
		p.mu.Lock()
		awaited, settled := p.awaited, p.settled
		p.mu.Unlock()
		if p.method == "" && !settled {
			continue
		}
		result, exc, err := p.wait(in)
		if err != nil {
			return nil, err
		}
		s := protogen.PromiseState{ID: id, Method: p.method, Awaited: awaited}
		if exc != nil {
			s.Failure = &protogen.Failure{Class: exc.Cls.QualifiedName(), Message: exc.Message, Line: exc.Line}
			// The whole exception, with its fields and cause, when the
			// envelope carries it.
			if data, err := enc.Encode(object.ExceptionValue(exc)); err == nil {
				s.Value = data
			}
		} else if s.Value, err = enc.Encode(result); err != nil {
			return nil, fmt.Errorf("promise %d (%s): %w", id, p.method, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// Resume readies the interpreter to continue the run d describes, whose
// script prog is: it runs the definitions among the top-level statements
// before the pause point — class, module, def, include and require — and
// restores the variables, the constants and the promises. Run then runs
// the statements from the pause point on.
func (in *Interp) Resume(prog *syntax.Program, d *protogen.Dump) error {
	if d.Next < 0 || d.Next > len(prog.Body) {
		return fmt.Errorf("the dump continues at statement %d of %d", d.Next, len(prog.Body))
	}
	in.file = d.File
	in.code.compiled = CompiledFor(prog)
	fr := in.topFrame()
	for _, n := range prog.Body[:d.Next] {
		if !isDefinition(n) {
			continue
		}
		if _, err := in.code.expr(n)(in, in.top, fr); err != nil {
			return err
		}
	}
	if len(d.Loaded) > 0 && in.loaded == nil {
		in.loaded = map[string]bool{}
	}
	for _, lib := range d.Loaded {
		in.loaded[lib] = true
	}
	dec := in.Decoder()
	for _, b := range d.Variables {
		v, ok, err := in.restore(dec, b)
		if err != nil {
			return fmt.Errorf("variable %s: %w", b.Name, err)
		}
		if ok {
			in.top.Define(b.Name, v)
		}
	}
	for _, b := range d.Constants {
		v, ok, err := in.restore(dec, b)
		if err != nil {
			return fmt.Errorf("constant %s: %w", b.Name, err)
		}
		if ok {
			in.consts[b.Name] = v
		}
	}
	in.changedTop()
	for _, s := range d.Promises {
		if err := in.restorePromise(dec, s); err != nil {
			return fmt.Errorf("promise %d: %w", s.ID, err)
		}
	}
	if d.Rand != nil {
		if err := in.rng.UnmarshalBinary(d.Rand); err != nil {
			// A dump of a cameo before Random followed CRuby's MT19937 holds
			// another generator's state.
			return fmt.Errorf("the dump's random number state is not one this cameo reads: %w", err)
		}
	}
	in.from = d.Next
	return nil
}

// Restore reads a dump binding's value back, once Resume brought the
// script's classes; false for a binding the dump left out.
func (in *Interp) Restore(b protogen.Binding) (object.Value, bool, error) {
	return in.restore(in.Decoder(), b)
}

func (in *Interp) restore(dec *protogen.Decoder, b protogen.Binding) (object.Value, bool, error) {
	switch {
	case b.Class != "":
		cls := in.classNamed(b.Class)
		if cls == nil {
			return object.Value{}, false, fmt.Errorf("no class %s", b.Class)
		}
		return object.ClassValue(cls), true, nil
	case b.Value != nil:
		v, err := dec.Decode(b.Value)
		return v, err == nil, err
	}
	return object.Value{}, false, nil
}

// classNamed finds a class by its qualified name among the script's
// constants and the builtin ones; nil when there is none.
func (in *Interp) classNamed(name string) *object.Class {
	find := func(consts map[string]object.Value) *object.Class {
		var cls *object.Class
		for _, part := range strings.Split(name, "::") {
			var ok bool
			if cls, ok = consts[part].AsClass(); !ok {
				return nil
			}
			consts = cls.Consts
		}
		return cls
	}
	if cls := find(in.consts); cls != nil {
		return cls
	}
	return find(object.ObjectClass.Consts)
}

// restorePromise puts a settled promise back into the run's table.
func (in *Interp) restorePromise(dec *protogen.Decoder, s protogen.PromiseState) error {
	p := &Promise{id: s.ID, method: s.Method, done: make(chan struct{})}
	var result object.Value
	var exc *object.Exception
	if s.Failure != nil {
		cls := in.classNamed(s.Failure.Class)
		if cls == nil {
			return fmt.Errorf("no class %s", s.Failure.Class)
		}
		exc = &object.Exception{Cls: cls, Message: s.Failure.Message, Line: s.Failure.Line, Fields: defaultFields(cls)}
		if len(s.Value) > 0 {
			v, err := dec.Decode(s.Value)
			if err != nil {
				return err
			}
			if e, ok := v.AsException(); ok {
				exc = e
			}
		}
	} else {
		var err error
		if result, err = dec.Decode(s.Value); err != nil {
			return err
		}
	}
	p.settle(result, exc, nil)
	p.awaited = s.Awaited
	in.promises.restore(p)
	return nil
}

// isDefinition reports a top-level statement a resumed run runs again: a
// class, module or method definition, include, or require.
func isDefinition(n syntax.Node) bool {
	switch n := n.(type) {
	case *syntax.ClassDef, *syntax.ModuleDef, *syntax.Def, *syntax.Include:
		return true
	case *syntax.Call:
		return n.Recv == nil && n.Name == "require"
	}
	return false
}
