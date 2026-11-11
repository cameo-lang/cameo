package eval

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A run pauses before an instruction, inside a statement or between two,
// when every activation it is running is on the interpreter's stack: one
// loop runs, and no builtin is in the middle of a call into the script
// (stack.go). What a dump holds, and how a resumed run gets its classes
// and its activations back, is in doc/design/pause.md.

// ErrPaused ends a run that Pause asked to pause. It is not an exception,
// so no rescue catches it.
var ErrPaused = errors.New("the script paused")

// Pause asks the run to pause before its next instruction: Run then
// returns ErrPaused, and Dump captures the run. A run that is inside a
// builtin's call into the script, the block of an each, pauses once the
// builtin returned; Stop ends a run at once.
func (in *Interp) Pause() { in.pause.Store(true) }

// Dump captures a paused run: the script, where it paused, its top-level
// variables, the constants it assigned, and its promises. It waits for the
// calls async started, so their results travel settled. Source is the
// script's text, which the dump carries for the run that resumes it.
func (in *Interp) Dump(source string) (*cameopb.Dump, error) {
	if !in.paused {
		return nil, errors.New("the script is not paused")
	}
	d := &cameopb.Dump{Version: ir.Version, File: in.file, Source: source, Next: int32(in.next), Line: int32(in.nextLine), CameoTop: in.cameoTop}
	// One encoder for the whole dump: a Proc travels as a closure, which
	// names its code in a unit of the dump.
	classes := in.newClassDump()
	if len(in.frozen) > 0 {
		// The run paused inside a statement: a Proc written in a frame
		// of its activations refers to the frame (EncodeProc).
		in.dumping = stackFrames(in.frozen)
		defer func() { in.dumping = nil }()
	}
	// The class of an instance is carried too: a class the script
	// reopened since it made the instance is one no constant leads to.
	enc := in.ProcEncoder(false).WithSymbols().WithClasses(func(cls *object.Class) {
		if classes.carried(cls) {
			classes.ref(cls)
		}
	})
	in.top.each(func(name string, v object.Value) {
		d.Variables = append(d.Variables, classes.binding(enc, name, v))
	})
	for _, name := range sortedKeys(in.consts) {
		v := in.consts[name]
		// A class Go defined or the host registered, under its own name,
		// is there in the run that resumes.
		if cls, ok := v.AsClass(); ok && cls.Name == name && !classes.carried(cls) {
			continue
		}
		d.Constants = append(d.Constants, classes.binding(enc, name, v))
	}
	promises, err := in.settledPromises(enc)
	if err != nil {
		return nil, err
	}
	d.Promises = promises
	for _, block := range in.exits.all() {
		d.AtExit = append(d.AtExit, classes.binding(enc, "", block))
	}
	if in.dumping != nil {
		if err := in.dumpStack(d, enc, classes); err != nil {
			return nil, fmt.Errorf("the run cannot be dumped where it paused: %w", err)
		}
	}
	d.Top = &cameopb.Dump_Class{Methods: classes.methods(in.mainClass)}
	d.Classes = classes.classes(enc)
	d.Missing = classes.missing
	// The units come last: a closure a worker made is of a unit the run
	// learned of only when the closure was read.
	in.code.units.Range(func(_, unit any) bool {
		d.Units = append(d.Units, unit.(*cameopb.Unit))
		return true
	})
	sort.Slice(d.Units, func(i, j int) bool { return d.Units[i].GetDigest() < d.Units[j].GetDigest() })
	for lib := range in.loaded {
		d.Loaded = append(d.Loaded, lib)
	}
	sort.Strings(d.Loaded)
	if d.Rand, err = in.rng.MarshalBinary(); err != nil {
		return nil, err
	}
	return d, nil
}

// settledPromises waits for every call async started and returns the
// promises of the run, settled. A promise the host or the script made by
// hand and has not settled is left out.
func (in *Interp) settledPromises(enc *protogen.Encoder) ([]*cameopb.Dump_Promise, error) {
	t := in.promises
	var out []*cameopb.Dump_Promise
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
		s := &cameopb.Dump_Promise{Id: id, Method: p.method, Awaited: awaited}
		if exc != nil {
			s.Failure = &cameopb.Dump_Failure{Class: exc.Cls.QualifiedName(), Message: exc.Message, Line: int32(exc.Line)}
			// The whole exception, with its fields and cause, when the
			// envelope carries it.
			if msg, err := enc.Message(object.ExceptionValue(exc)); err == nil {
				s.Value = msg
			}
		} else if s.Value, err = enc.Message(result); err != nil {
			return nil, fmt.Errorf("promise %d (%s): %w", id, p.method, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// Resume readies the interpreter to continue the run d describes, whose
// script prog is: it makes the classes of the dump, restores the
// variables, the constants and the promises, and the activations of a
// run that paused inside a statement. Run then goes on from the pause
// point.
func (in *Interp) Resume(prog *syntax.Program, d *cameopb.Dump) error {
	if d.GetVersion() != ir.Version {
		return fmt.Errorf("the dump is of IR version %d, and this cameo runs version %d: a dump resumes on a cameo of its version", d.GetVersion(), ir.Version)
	}
	next := int(d.Next)
	if next < 0 || next > len(prog.Body) {
		return fmt.Errorf("the dump continues at statement %d of %d", next, len(prog.Body))
	}
	in.file = d.File
	in.code.compiled = CompiledFor(prog)
	// The script's own unit first, so that the code the run continues in
	// and the code of its methods are one.
	script, err := in.program(d.File, prog)
	if err != nil {
		return err
	}
	if err := in.restoreUnits(d.GetUnits()); err != nil {
		return err
	}
	if len(d.Loaded) > 0 && in.loaded == nil {
		in.loaded = map[string]bool{}
	}
	for _, lib := range d.Loaded {
		in.loaded[lib] = true
		in.bringToTop(lib)
	}
	if d.GetCameoTop() && !in.cameoTop {
		in.mainClass.Include(object.CameoModule)
		in.cameoTop = true
	}
	if err := in.restoreClasses(d); err != nil {
		return err
	}
	// The frames of a run that paused inside a statement, before any
	// value is read: a Proc among the values may refer to one.
	dec := in.Decoder()
	if err := in.restoreShells(dec, d); err != nil {
		return err
	}
	if err := in.restoreClassValues(dec, d.GetClasses()); err != nil {
		return err
	}
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
			return fmt.Errorf("promise %d: %w", s.Id, err)
		}
	}
	for i, b := range d.AtExit {
		v, ok, err := in.restore(dec, b)
		if err != nil {
			return fmt.Errorf("at_exit block %d: %w", i+1, err)
		}
		if ok {
			in.exits.push(v)
		}
	}
	if d.Rand != nil {
		if err := in.rng.UnmarshalBinary(d.Rand); err != nil {
			// A dump of a cameo before Random followed CRuby's MT19937 holds
			// another generator's state.
			return fmt.Errorf("the dump's random number state is not one this cameo reads: %w", err)
		}
	}
	if err := in.restoreStack(dec, d, script); err != nil {
		return err
	}
	in.from = next
	return nil
}

// Restore reads a dump binding's value back, once Resume brought the
// script's classes; false for a binding the dump left out.
func (in *Interp) Restore(b *cameopb.Dump_Binding) (object.Value, bool, error) {
	return in.restore(in.Decoder(), b)
}

func (in *Interp) restore(dec *protogen.Decoder, b *cameopb.Dump_Binding) (object.Value, bool, error) {
	switch {
	case b.Main:
		return object.InstanceValue(in.main), true, nil
	case b.Building != 0:
		if in.resuming == nil || int(b.Building) > len(in.resuming.frames) || in.resuming.frames[b.Building-1].Building == nil {
			return object.Value{}, false, fmt.Errorf("the dump has no instance being made in frame %d", b.Building-1)
		}
		return object.InstanceValue(in.resuming.frames[b.Building-1].Building), true, nil
	case b.Class != "":
		cls := in.classRef(b.Class)
		if cls == nil {
			return object.Value{}, false, fmt.Errorf("no class %s", b.Class)
		}
		return object.ClassValue(cls), true, nil
	case b.Value != nil:
		v, err := dec.Value(b.Value)
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
func (in *Interp) restorePromise(dec *protogen.Decoder, s *cameopb.Dump_Promise) error {
	p := &Promise{id: s.Id, method: s.Method, done: make(chan struct{})}
	var result object.Value
	var exc *object.Exception
	if s.Failure != nil {
		cls := in.classNamed(s.Failure.Class)
		if cls == nil {
			return fmt.Errorf("no class %s", s.Failure.Class)
		}
		exc = &object.Exception{Cls: cls, Message: s.Failure.Message, Line: int(s.Failure.Line), Fields: defaultFields(cls)}
		if s.Value != nil {
			v, err := dec.Value(s.Value)
			if err != nil {
				return err
			}
			if e, ok := v.AsException(); ok {
				exc = e
			}
		}
	} else {
		var err error
		if result, err = dec.Value(s.Value); err != nil {
			return err
		}
	}
	p.settle(result, exc, nil)
	p.awaited = s.Awaited
	in.promises.restore(p)
	return nil
}
