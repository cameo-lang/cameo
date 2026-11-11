package eval

import (
	"fmt"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// The activations of a run in its dump, and back: a run that paused inside
// a statement resumes inside it. doc/design/ir.md, "The activation stack",
// has the design, and dump.proto the messages.

// stackDump numbers the frames and scopes of a paused run's activations:
// they are shared, a block's activation and its method's have one frame,
// so the dump holds each once and refers to it by its index.
type stackDump struct {
	frames    map[*Frame]uint32
	frameList []*Frame
	scopes    map[*Env]uint32
	scopeList []*Env
	// building holds the instances the frames' initializes are filling,
	// each with the index of its frame: an instance that still changes
	// is in the dump once, and referred to there.
	building map[*object.Instance]uint32
}

// buildingIn returns the index, plus one, of the frame whose initialize
// is filling the instance v is, and zero when v is no such instance.
func (sd *stackDump) buildingIn(v object.Value) uint32 {
	if sd == nil {
		return 0
	}
	if inst, ok := v.AsInstance(); ok {
		if i, ok := sd.building[inst]; ok {
			return i + 1
		}
	}
	return 0
}

// stackMsg is the stack of one interpreter as a dump holds it: the
// script's, in the fields of the Dump, or a worker's, in a Worker. next
// is the statement the script's first activation is in.
type stackMsg struct {
	stack  []*cameopb.Dump_Activation
	scopes []*cameopb.Dump_Scope
	frames []*cameopb.Dump_Frame
	next   int
}

// stackFrames numbers the frames and scopes of the activations acts.
func stackFrames(acts []*machine) *stackDump {
	sd := &stackDump{frames: map[*Frame]uint32{}, scopes: map[*Env]uint32{}, building: map[*object.Instance]uint32{}}
	for _, m := range acts {
		if m.kind == actStep {
			// A builtin has neither.
			continue
		}
		if _, ok := sd.frames[m.fr]; !ok {
			sd.frames[m.fr] = uint32(len(sd.frameList))
			sd.frameList = append(sd.frameList, m.fr)
			// The first frame that fills an instance holds it; the frame
			// of an initialize its super called refers to that one.
			if inst := m.fr.Building; inst != nil {
				if _, held := sd.building[inst]; !held {
					sd.building[inst] = sd.frames[m.fr]
				}
			}
		}
		sd.scope(m.env)
	}
	return sd
}

// scope numbers env and the scopes it refers to.
func (sd *stackDump) scope(env *Env) {
	if _, ok := sd.scopes[env]; ok || env == nil {
		return
	}
	sd.scopes[env] = uint32(len(sd.scopeList))
	sd.scopeList = append(sd.scopeList, env)
	sd.scope(env.parent)
	sd.scope(env.scope)
}

// ref is the reference to a scope: its index plus one, zero for none.
func (sd *stackDump) ref(env *Env) uint32 {
	if env == nil {
		return 0
	}
	return sd.scopes[env] + 1
}

// frameOf returns the index of fr plus one when fr is a frame of the
// paused run's activations, and zero otherwise.
func (sd *stackDump) frameOf(fr *Frame) uint32 {
	if sd == nil {
		return 0
	}
	if i, ok := sd.frames[fr]; ok {
		return i + 1
	}
	return 0
}

// dumpStack writes the activations the run paused in, with their frames
// and scopes, into d.
func (in *Interp) dumpStack(d *stackMsg, enc *protogen.Encoder, classes *classDump) error {
	sd, acts := in.dumping, in.frozen
	value := func(what string, v object.Value) *cameopb.Dump_Binding {
		if v.IsZero() {
			return &cameopb.Dump_Binding{}
		}
		b := classes.binding(enc, "", v)
		if b.Missing != "" {
			classes.missing = append(classes.missing, fmt.Sprintf("%s (%s)", what, b.Missing))
		}
		return b
	}
	for i, m := range acts {
		where := in.actName(m)
		a := &cameopb.Dump_Activation{Line: int32(m.line())}
		if m.kind != actStep {
			a.Scope, a.Frame = sd.scopes[m.env], sd.frames[m.fr]
		}
		switch m.kind {
		case actStep:
			st := m.stepper
			a.Kind = cameopb.Dump_Activation_BUILTIN
			b := &cameopb.Dump_Builtin{Method: in.methodRef(classes, st.builtin), Rescue: st.rescue}
			b.Self = value("self of "+where, st.call.Self)
			for _, arg := range st.call.Args {
				b.Args = append(b.Args, value("an argument of "+where, arg))
			}
			if st.call.Kw != nil {
				kw, err := enc.Message(object.MapValue(st.call.Kw))
				if err != nil {
					return fmt.Errorf("the keywords of %s: %w", where, err)
				}
				b.Kw = kw
			}
			if b.Block = value("the block of "+where, st.call.Block); b.Block.Missing != "" {
				return fmt.Errorf("the block of %s: %s", where, b.Block.Missing)
			}
			for _, v := range st.state {
				b.State = append(b.State, value("what "+where+" keeps", v))
			}
			a.Builtin = b
		case actMethod:
			a.Kind = cameopb.Dump_Activation_METHOD
			a.Unit, a.Proc = procRef(m.fr.Method.Body.(*irProc))
		case actBlock:
			a.Kind = cameopb.Dump_Activation_BLOCK
			a.Unit, a.Proc = procRef(m.proc.IR)
			// A block's activation is of the block the activation below
			// yielded to, where the resume finds the very same proc; or
			// of a proc called as a value, which the dump holds here.
			if i == 0 {
				return fmt.Errorf("the block %s runs was called by nothing", where)
			}
			if blockTarget(acts[i-1].yields()) != any(m.proc) {
				if a.Block = value("the proc of "+where, procValue(m.proc)); a.Block.Missing != "" {
					return fmt.Errorf("the proc of %s: %s", where, a.Block.Missing)
				}
			}
		default:
			if i != 0 {
				return fmt.Errorf("a list runs inside %s that is no statement of the top level", where)
			}
		}
		for _, cur := range m.cursors {
			c := &cameopb.Dump_Cursor{Next: uint32(cur.next), List: uint32(cur.list), Phase: uint32(cur.phase), Base: uint32(cur.base), Statement: cur.statement}
			if cur.exc != nil {
				msg, err := enc.Message(object.ExceptionValue(cur.exc))
				if err != nil {
					return fmt.Errorf("the exception %s is rescuing: %w", where, err)
				}
				c.Rescuing = msg
			}
			if cur.pending != nil {
				pending, err := in.dumpCompletion(cur.pending, enc, classes, sd)
				if err != nil {
					return fmt.Errorf("what an ensure clause of %s interrupted: %w", where, err)
				}
				c.Pending = pending
			}
			a.Cursors = append(a.Cursors, c)
		}
		for _, v := range m.stack {
			a.Stack = append(a.Stack, value("a value in flight in "+where, v))
		}
		if !m.result.IsZero() {
			a.Result = value("what "+where+" returns", m.result)
		}
		if w := m.wait; w != nil {
			aw := &cameopb.Dump_Awaiting{Single: w.single, Next: uint32(w.next)}
			for _, p := range w.ps {
				aw.Promises = append(aw.Promises, p.id)
			}
			for _, v := range w.results {
				aw.Results = append(aw.Results, value("a result "+where+" awaited", v))
			}
			if w.failed != nil {
				msg, err := enc.Message(object.ExceptionValue(w.failed))
				if err != nil {
					return fmt.Errorf("the failure %s awaited: %w", where, err)
				}
				aw.Failed = msg
			}
			a.Awaiting = aw
		}
		d.stack = append(d.stack, a)
	}
	for i, fr := range sd.frameList {
		if fr.BuildingExc != nil || fr.Reopened != nil {
			return fmt.Errorf("frame %d is of an exception's initialize or a class body, which no pause is inside", i)
		}
		// The top level's frame is the frame of the statement the
		// script paused in; a worker's stack has none.
		top := acts[0].kind == actList && fr == acts[0].fr
		f := &cameopb.Dump_Frame{Top: top, File: fr.File, Lambda: fr.Lambda, Ended: fr.ended}
		if inst := fr.Building; inst != nil && sd.building[inst] != uint32(i) {
			f.BuildingOf = sd.building[inst] + 1
		} else if inst != nil {
			// The instance as it is now. Its message is not kept on it:
			// its initialize goes on filling it.
			msg, err := enc.Message(object.InstanceValue(inst))
			inst.Message = nil
			if err != nil {
				return fmt.Errorf("the instance frame %d is making: %w", i, err)
			}
			f.Building = msg
		}
		what := fmt.Sprintf("of frame %d", i)
		f.Self = value("self "+what, fr.Self)
		if m := fr.Method; m != nil {
			f.Method = in.methodRef(classes, m)
		}
		for _, a := range fr.Args {
			f.Args = append(f.Args, value("an argument "+what, a))
		}
		if fr.Kw != nil {
			kw, err := enc.Message(object.MapValue(fr.Kw))
			if err != nil {
				classes.missing = append(classes.missing, fmt.Sprintf("the keywords %s (%s)", what, err))
			}
			f.Kw = kw
		}
		if !fr.Block.IsZero() {
			f.Block = value("the block "+what, fr.Block)
		}
		for _, cls := range fr.Lexical {
			f.Lexical = append(f.Lexical, classes.ref(cls))
		}
		f.DefTarget = classes.ref(fr.DefTarget)
		d.frames = append(d.frames, f)
	}
	for i, env := range sd.scopeList {
		s := &cameopb.Dump_Scope{Top: env == in.top, Parent: sd.ref(env.parent), Captured: env.captured, Origin: sd.ref(env.scope)}
		if !s.Top {
			flat := map[string]object.Value{}
			env.each(func(name string, v object.Value) { flat[name] = v })
			for _, name := range sortedKeys(flat) {
				b := classes.binding(enc, name, flat[name])
				if b.Missing != "" {
					classes.missing = append(classes.missing, fmt.Sprintf("%s of scope %d (%s)", name, i, b.Missing))
				}
				s.Locals = append(s.Locals, b)
			}
		}
		for _, p := range env.promises {
			s.Promises = append(s.Promises, p.id)
		}
		d.scopes = append(d.scopes, s)
	}
	return nil
}

// methodRef names the method m as a dump does: by the class that defines
// it, "" for the top level, and whether it is a class method.
func (in *Interp) methodRef(classes *classDump, m *object.Method) *cameopb.Binding_Method {
	ref := &cameopb.Binding_Method{Name: m.Name}
	if m.Owner != nil && m.Owner != in.mainClass {
		ref.Owner = classes.ref(m.Owner)
		ref.Singleton = m.Owner.SingletonMethods[m.Name] == m
	}
	return ref
}

// method finds the method a dump names with ref.
func (in *Interp) method(ref *cameopb.Binding_Method) (*object.Method, error) {
	owner := in.mainClass
	if ref.GetOwner() != "" {
		if owner = in.classRef(ref.GetOwner()); owner == nil {
			return nil, fmt.Errorf("no class %s", ref.GetOwner())
		}
	}
	m := owner.Methods[ref.GetName()]
	if ref.GetSingleton() {
		m = owner.SingletonMethods[ref.GetName()]
	}
	if m == nil {
		return nil, fmt.Errorf("no method %s in %s", ref.GetName(), owner.QualifiedName())
	}
	return m, nil
}

// yields returns the block the activation yields to: the block of its
// frame, or for a builtin the block it was called with.
func (m *machine) yields() object.Value {
	if m.kind == actStep {
		return m.stepper.call.Block
	}
	return m.fr.Block
}

// procRef names the proc p as a dump does: the digest of its unit and its
// index in the unit.
func procRef(p *irProc) (string, uint32) {
	for i, proc := range p.unit.GetProcs() {
		if proc == p.proc {
			return p.unit.GetDigest(), uint32(i)
		}
	}
	return p.unit.GetDigest(), 0
}

// actName names an activation for a report: its method, or the block or
// the statement it is.
func (in *Interp) actName(m *machine) string {
	switch {
	case m.kind == actStep:
		return methodName(m.stepper.builtin, m.stepper.call.Self)
	case m.kind == actMethod:
		return methodName(m.fr.Method, m.fr.Self)
	case m.kind == actBlock && m.fr.Method != nil:
		return "a block in " + methodName(m.fr.Method, m.fr.Self)
	case m.kind == actBlock:
		return "a block"
	}
	return "the statement"
}

// line is the line of the activation's next instruction: the innermost
// list that has one.
func (m *machine) line() int {
	if len(m.cursors) == 0 {
		return 0
	}
	for i := len(m.cursors) - 1; i >= 0; i-- {
		if cur := m.cursors[i]; cur.next < len(cur.code) {
			return int(cur.code[cur.next].Line)
		}
	}
	if cur := m.cursors[0]; len(cur.code) > 0 {
		return int(cur.code[len(cur.code)-1].Line)
	}
	return 0
}

// dumpCompletion is the dump's form of a completion an ensure clause
// keeps while it runs.
func (in *Interp) dumpCompletion(err error, enc *protogen.Encoder, classes *classDump, sd *stackDump) (*cameopb.Dump_Completion, error) {
	if exc, ok := err.(*object.Exception); ok {
		msg, err := enc.Message(object.ExceptionValue(exc))
		if err != nil {
			return nil, err
		}
		return &cameopb.Dump_Completion{Kind: cameopb.Dump_Completion_EXCEPTION, Exception: msg}, nil
	}
	ctl, ok := err.(*control)
	if !ok {
		return nil, fmt.Errorf("%v is not in a dump", err)
	}
	c := &cameopb.Dump_Completion{Line: int32(ctl.line)}
	if !ctl.value.IsZero() {
		if c.Value = classes.binding(enc, "", ctl.value); c.Value.Missing != "" {
			return nil, fmt.Errorf("its value: %s", c.Value.Missing)
		}
	}
	switch ctl.kind {
	case ctrlBreak:
		c.Kind = cameopb.Dump_Completion_BREAK
		// A break out of a block is on its way to the call the block
		// was given to.
		if p, ok := ctl.target.(*Proc); ok {
			for i, fr := range sd.frameList {
				if blockTarget(fr.Block) == any(p) {
					c.Target = uint32(i) + 1
				}
			}
			if c.Target == 0 {
				return nil, fmt.Errorf("a break out of a block no running call was given")
			}
		}
	case ctrlNext:
		c.Kind = cameopb.Dump_Completion_NEXT
	case ctrlReturn:
		c.Kind = cameopb.Dump_Completion_RETURN
		fr, _ := ctl.target.(*Frame)
		if c.Target = sd.frameOf(fr); c.Target == 0 {
			return nil, fmt.Errorf("a return from a call that is not running")
		}
	case ctrlRetry:
		c.Kind = cameopb.Dump_Completion_RETRY
	case ctrlRedo:
		c.Kind = cameopb.Dump_Completion_REDO
	default:
		return nil, fmt.Errorf("a throw is not in a dump")
	}
	return c, nil
}

// stackRestore is the frames and scopes of a dump while the run that
// resumes it is made: a closure and an activation refer to them by index.
type stackRestore struct {
	frames []*Frame
	scopes []*Env
}

// restoreShells makes the frames and scopes of a dump, empty, before any
// value of the dump is read: a closure among the values may refer to a
// frame.
func (in *Interp) restoreShells(dec *protogen.Decoder, d *stackMsg) error {
	if len(d.stack) == 0 {
		return nil
	}
	sr := &stackRestore{}
	for _, f := range d.frames {
		fr := &Frame{}
		if f.GetTop() {
			fr = in.topFrame()
		}
		sr.frames = append(sr.frames, fr)
	}
	for _, s := range d.scopes {
		env := &Env{}
		if s.GetTop() {
			env = in.top
		}
		sr.scopes = append(sr.scopes, env)
	}
	in.resuming = sr
	// The instances that initializes are filling, which other values of
	// the dump refer to by their frame.
	for i, f := range d.frames {
		if f.GetBuilding() == nil {
			continue
		}
		v, err := dec.Value(f.GetBuilding())
		if err != nil {
			return fmt.Errorf("the instance frame %d is making: %w", i, err)
		}
		inst, ok := v.AsInstance()
		if !ok {
			return fmt.Errorf("frame %d of the dump is making what is no instance", i)
		}
		// Its initialize goes on filling it: no message is kept on it.
		inst.Message = nil
		sr.frames[i].Building = inst
	}
	for i, f := range d.frames {
		if ref := f.GetBuildingOf(); ref != 0 {
			if int(ref) > len(sr.frames) || sr.frames[ref-1].Building == nil {
				return fmt.Errorf("frame %d of the dump fills the instance of frame %d, which makes none", i, ref-1)
			}
			sr.frames[i].Building = sr.frames[ref-1].Building
		}
	}
	return nil
}

// restoreStack fills the frames and scopes of a dump and makes its
// activations, which Run continues.
func (in *Interp) restoreStack(dec *protogen.Decoder, d *stackMsg, script *program) error {
	sr := in.resuming
	if sr == nil {
		return nil
	}
	value := func(b *cameopb.Dump_Binding) (object.Value, error) {
		v, _, err := in.restore(dec, b)
		return v, err
	}
	frame := func(ref uint32) (*Frame, error) {
		if ref == 0 || int(ref) > len(sr.frames) {
			return nil, fmt.Errorf("the dump has no frame %d", int(ref)-1)
		}
		return sr.frames[ref-1], nil
	}
	scope := func(ref uint32) (*Env, error) {
		if ref == 0 {
			return nil, nil
		}
		if int(ref) > len(sr.scopes) {
			return nil, fmt.Errorf("the dump has no scope %d", int(ref)-1)
		}
		return sr.scopes[ref-1], nil
	}
	class := func(ref string) (*object.Class, error) {
		if cls := in.classRef(ref); cls != nil {
			return cls, nil
		}
		return nil, fmt.Errorf("no class %s", ref)
	}
	for i, f := range d.frames {
		fr := sr.frames[i]
		var err error
		if f.GetSelf() != nil {
			if fr.Self, err = value(f.GetSelf()); err != nil {
				return fmt.Errorf("self of frame %d: %w", i, err)
			}
		}
		if ref := f.GetMethod(); ref != nil {
			owner := in.mainClass
			if ref.GetOwner() != "" {
				if owner, err = class(ref.GetOwner()); err != nil {
					return err
				}
			}
			m := owner.Methods[ref.GetName()]
			if ref.GetSingleton() {
				m = owner.SingletonMethods[ref.GetName()]
			}
			if m == nil {
				return fmt.Errorf("frame %d is of %s in %s, which is not there", i, ref.GetName(), owner.QualifiedName())
			}
			fr.Method, fr.Owner = m, m.Owner
		}
		for _, a := range f.GetArgs() {
			v, err := value(a)
			if err != nil {
				return fmt.Errorf("an argument of frame %d: %w", i, err)
			}
			fr.Args = append(fr.Args, v)
		}
		if f.GetKw() != nil {
			kw, err := dec.Value(f.GetKw())
			if err != nil {
				return fmt.Errorf("the keywords of frame %d: %w", i, err)
			}
			fr.Kw = kw.Map()
		}
		if f.GetBlock() != nil {
			if fr.Block, err = value(f.GetBlock()); err != nil {
				return fmt.Errorf("the block of frame %d: %w", i, err)
			}
		}
		for _, ref := range f.GetLexical() {
			cls, err := class(ref)
			if err != nil {
				return err
			}
			fr.Lexical = append(fr.Lexical, cls)
		}
		if ref := f.GetDefTarget(); ref != "" {
			if fr.DefTarget, err = class(ref); err != nil {
				return err
			}
		} else {
			fr.DefTarget = in.mainClass
		}
		fr.File, fr.Lambda, fr.ended = f.GetFile(), f.GetLambda(), f.GetEnded()
	}
	for i, s := range d.scopes {
		env := sr.scopes[i]
		var err error
		if !s.GetTop() {
			for _, b := range s.GetLocals() {
				v, ok, err := in.restore(dec, b)
				if err != nil {
					return fmt.Errorf("%s of scope %d: %w", b.GetName(), i, err)
				}
				if ok {
					env.bind(b.GetName(), v)
				}
			}
			env.captured = s.GetCaptured()
		}
		if env.parent, err = scope(s.GetParent()); err != nil {
			return err
		}
		if env.scope, err = scope(s.GetOrigin()); err != nil {
			return err
		}
		for _, id := range s.GetPromises() {
			if p := in.promises.get(id); p != nil {
				env.promises = append(env.promises, p)
			}
		}
	}
	var acts []*machine
	// rescuing is the exception the innermost rescue clause so far is
	// handling, as the run had it in Interp.rescuing.
	var rescuing *object.Exception
	for i, a := range d.stack {
		m := &machine{in: in}
		var err error
		// site is the instruction of the activation below that called;
		// a builtin calls its block from no instruction.
		if i > 0 && acts[i-1].kind != actStep {
			below := acts[i-1].cursors[len(acts[i-1].cursors)-1]
			if below.next == 0 {
				return fmt.Errorf("activation %d of the dump was called by no instruction", i)
			}
			m.site = below.code[below.next-1]
		}
		if a.GetKind() == cameopb.Dump_Activation_BUILTIN {
			b := a.GetBuiltin()
			meth, err := in.method(b.GetMethod())
			if err != nil {
				return fmt.Errorf("activation %d of the dump: %w", i, err)
			}
			if meth.Step == nil || i == 0 {
				return fmt.Errorf("activation %d of the dump is inside %s, which this cameo cannot resume inside", i, meth.Name)
			}
			st := &stepping{builtin: meth, call: &object.Call{}, rescue: b.GetRescue()}
			if st.call.Self, err = value(b.GetSelf()); err != nil {
				return err
			}
			for _, arg := range b.GetArgs() {
				v, err := value(arg)
				if err != nil {
					return err
				}
				st.call.Args = append(st.call.Args, v)
			}
			if b.GetKw() != nil {
				kw, err := dec.Value(b.GetKw())
				if err != nil {
					return err
				}
				st.call.Kw = kw.Map()
			}
			if st.call.Block, err = value(b.GetBlock()); err != nil {
				return err
			}
			for _, s := range b.GetState() {
				v, err := value(s)
				if err != nil {
					return err
				}
				st.state = append(st.state, v)
			}
			m.kind, m.stepper = actStep, st
			acts = append(acts, m)
			continue
		}
		if m.fr, err = frame(a.GetFrame() + 1); err != nil {
			return err
		}
		if m.env, err = scope(a.GetScope() + 1); err != nil {
			return err
		}
		var code []*cameopb.Code
		switch a.GetKind() {
		case cameopb.Dump_Activation_STATEMENT:
			if i != 0 || script == nil || d.next >= len(script.statements) {
				return fmt.Errorf("activation %d of the dump is a statement the script does not have", i)
			}
			code = script.statements[d.next]
			m.unit = script.unit
		default:
			known, ok := in.code.units.Load(a.GetUnit())
			if !ok || int(a.GetProc()) >= len(known.(*cameopb.Unit).GetProcs()) {
				return fmt.Errorf("the code of activation %d, proc %d of unit %s, is not in the dump", i, a.GetProc(), a.GetUnit())
			}
			m.unit = known.(*cameopb.Unit)
			code = m.unit.Procs[a.GetProc()].GetCode()
			m.kind = actMethod
			if a.GetKind() == cameopb.Dump_Activation_BLOCK {
				m.kind = actBlock
				if i == 0 {
					return fmt.Errorf("activation 0 of the dump is a block")
				}
				block := acts[i-1].yields()
				if a.GetBlock() != nil {
					if block, err = value(a.GetBlock()); err != nil {
						return fmt.Errorf("the proc of activation %d: %w", i, err)
					}
				}
				p, ok := asProc(block)
				if !ok || !p.written() {
					return fmt.Errorf("activation %d of the dump is a block, and the dump has no proc for it", i)
				}
				m.proc = p
			}
		}
		for k, c := range a.GetCursors() {
			cur := cursor{next: int(c.GetNext()), list: int(c.GetList()), phase: phase(c.GetPhase()), base: int(c.GetBase()), statement: c.GetStatement()}
			if k == 0 {
				cur.code = code
			} else {
				prev := m.cursors[k-1]
				if prev.next == 0 || prev.next > len(prev.code) {
					return fmt.Errorf("cursor %d of activation %d is inside no instruction", k, i)
				}
				cur.owner = prev.code[prev.next-1]
				if cur.list >= len(cur.owner.GetCode()) {
					return fmt.Errorf("cursor %d of activation %d is in list %d of a %s", k, i, cur.list, cur.owner.GetOp())
				}
				cur.code = cur.owner.Code[cur.list].GetCode()
			}
			if cur.next > len(cur.code) {
				return fmt.Errorf("cursor %d of activation %d is past its list", k, i)
			}
			if c.GetRescuing() != nil {
				v, err := dec.Value(c.GetRescuing())
				if err != nil {
					return err
				}
				cur.exc, _ = v.AsException()
				if cur.phase == phaseRescue {
					cur.outer, rescuing = rescuing, cur.exc
				}
			}
			if c.GetPending() != nil {
				if cur.pending, err = in.restoreCompletion(dec, c.GetPending(), sr); err != nil {
					return err
				}
			}
			m.cursors = append(m.cursors, cur)
		}
		if len(m.cursors) == 0 {
			return fmt.Errorf("activation %d of the dump is nowhere", i)
		}
		for _, b := range a.GetStack() {
			v, err := value(b)
			if err != nil {
				return fmt.Errorf("a value in flight in activation %d: %w", i, err)
			}
			m.stack = append(m.stack, v)
		}
		if a.GetResult() != nil {
			if m.result, err = value(a.GetResult()); err != nil {
				return fmt.Errorf("what activation %d returns: %w", i, err)
			}
		}
		if aw := a.GetAwaiting(); aw != nil {
			w := &awaiting{single: aw.GetSingle(), next: int(aw.GetNext())}
			for k, id := range aw.GetPromises() {
				p := in.promises.get(id)
				// A promise the activation has the result of may be
				// gone; one it still waits for is in the dump.
				if p == nil && k >= w.next {
					return fmt.Errorf("activation %d of the dump awaits promise %d, which the dump does not have", i, id)
				}
				w.ps = append(w.ps, p)
			}
			for _, b := range aw.GetResults() {
				v, err := value(b)
				if err != nil {
					return fmt.Errorf("a result activation %d awaited: %w", i, err)
				}
				w.results = append(w.results, v)
			}
			if aw.GetFailed() != nil {
				v, err := dec.Value(aw.GetFailed())
				if err != nil {
					return err
				}
				w.failed, _ = v.AsException()
			}
			if w.next > len(w.ps) {
				return fmt.Errorf("activation %d of the dump awaits past its promises", i)
			}
			m.wait = w
		}
		acts = append(acts, m)
	}
	in.resumed, in.rescuing = acts, rescuing
	return nil
}

// restoreCompletion reads a completion an ensure clause keeps.
func (in *Interp) restoreCompletion(dec *protogen.Decoder, c *cameopb.Dump_Completion, sr *stackRestore) (error, error) {
	if c.GetKind() == cameopb.Dump_Completion_EXCEPTION {
		v, err := dec.Value(c.GetException())
		if err != nil {
			return nil, err
		}
		exc, ok := v.AsException()
		if !ok {
			return nil, fmt.Errorf("a pending exception of the dump is no exception")
		}
		return exc, nil
	}
	ctl := &control{line: int(c.GetLine())}
	if c.GetValue() != nil {
		v, _, err := in.restore(dec, c.GetValue())
		if err != nil {
			return nil, err
		}
		ctl.value = v
	}
	var target *Frame
	if t := c.GetTarget(); t != 0 {
		if int(t) > len(sr.frames) {
			return nil, fmt.Errorf("the dump has no frame %d", t-1)
		}
		target = sr.frames[t-1]
	}
	switch c.GetKind() {
	case cameopb.Dump_Completion_BREAK:
		ctl.kind = ctrlBreak
		if target != nil {
			p, ok := asProc(target.Block)
			if !ok {
				return nil, fmt.Errorf("a pending break of the dump leaves the block of a call that has none")
			}
			ctl.target = p
		}
	case cameopb.Dump_Completion_NEXT:
		ctl.kind = ctrlNext
	case cameopb.Dump_Completion_RETURN:
		ctl.kind = ctrlReturn
		if target == nil {
			return nil, fmt.Errorf("a pending return of the dump returns from no call")
		}
		ctl.target = target
	case cameopb.Dump_Completion_RETRY:
		ctl.kind = ctrlRetry
	case cameopb.Dump_Completion_REDO:
		ctl.kind = ctrlRedo
	}
	return ctl, nil
}
