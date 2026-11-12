package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// The activations of an interpreter: one stack of them, and one loop that
// runs the top one. A call of a method or a block a script wrote pushes an
// activation and goes back to the loop; a completion no cursor of an
// activation takes leaves the activation and goes on in the one below.
// doc/design/ir.md, "The activation stack", has the design.

// actKind says what an activation is, which decides what happens when it
// ends.
type actKind int

const (
	// actList is a list run for its value: a top-level statement, a
	// class body, a parameter's default.
	actList actKind = iota
	// actMethod is a call of a method a script defined.
	actMethod
	// actBlock is a call of a block or a lambda a script wrote.
	actBlock
	// actStep is a call of a builtin that is a stepper (object.Stepper):
	// it has no code, and runs a step between two calls of its block.
	actStep
)

// maxDepth is how many activations an interpreter runs at once before a
// call raises SystemStackError.
const maxDepth = 200_000

// start runs the activation m to its end and returns its value: the way
// into the evaluator for Go code, a builtin that calls a block or the
// host that calls a method. The loop it runs is nested in whatever loop
// called the builtin.
func (in *Interp) start(m *machine) (object.Value, error) {
	floor := len(in.acts)
	if err := in.pushAct(m, nil); err != nil {
		return object.Value{}, err
	}
	in.loops++
	defer func() {
		in.loops--
		in.acts = in.acts[:floor]
	}()
	return in.run(floor)
}

// atInstruction reports whether the activation's next step runs an
// instruction, and not the end of a list.
func (m *machine) atInstruction() bool {
	if len(m.cursors) == 0 {
		return m.atReal()
	}
	cur := &m.cursors[len(m.cursors)-1]
	return cur.next < len(cur.code)
}

// goOn runs the activations of the dump a run resumed from, from where
// they paused, and returns the value of the statement they are in.
func (in *Interp) goOn() (object.Value, error) {
	in.acts, in.resumed = in.resumed, nil
	in.loops++
	defer func() {
		in.loops--
		in.acts = in.acts[:0]
	}()
	return in.run(0)
}

// pushAct makes m the activation that runs. site is the instruction of
// the activation below that called, nil when Go code did.
func (in *Interp) pushAct(m *machine, site *cameopb.Code) error {
	if len(in.acts) >= maxDepth {
		_, err := in.leave(m, object.Value{}, in.raise(object.SystemStackError, 0, "stack level too deep"))
		return err
	}
	m.site = site
	in.acts = append(in.acts, m)
	return nil
}

// run runs the activations above floor until the one at floor ended, and
// returns its value or the completion that ended it.
func (in *Interp) run(floor int) (object.Value, error) {
	var err error
	for {
		m := in.acts[len(in.acts)-1]
		if m.kind == actStep {
			// A builtin that is a stepper: its next step, which calls
			// the block or ends the builtin.
			v, called, stepErr := in.stepOn(m, err)
			if called {
				err = nil
				continue
			}
			in.acts = in.acts[:len(in.acts)-1]
			if err = stepErr; len(in.acts) == floor {
				return v, err
			}
			if err != nil {
				err = withLine(err, int(m.site.Line))
				continue
			}
			in.acts[len(in.acts)-1].push(v)
			continue
		}
		if m.flat != nil && len(m.cursors) == 0 && err == nil && m.pc < len(m.flat.code) {
			// Flat code runs until the activation ends, calls, or
			// reaches an instruction a pause stops before.
			if in.pausing.flag.Load() && in.canPause(m) && m.atReal() && in.freeze() {
				return object.Value{}, ErrPaused
			}
			if err = m.runFlat(); err != nil || in.acts[len(in.acts)-1] != m || m.pc < len(m.flat.code) {
				continue
			}
		}
		if len(m.cursors) > 0 {
			if err == nil {
				// A pause stops the run here, before an instruction,
				// when this loop is the only one: every activation is
				// then on the stack, which Dump writes as it is. A list
				// that is through ends first, so that a statement that
				// is through pauses as one that has not begun.
				if in.pausing.flag.Load() && in.canPause(m) && m.atInstruction() && in.freeze() {
					return object.Value{}, ErrPaused
				}
				// An activation that awaits goes on waiting: one of a
				// resumed run, or of a run that went on after a pause.
				if m.wait != nil {
					err = m.awaited()
				} else {
					err = m.step()
				}
				// A pause woke the activation where it waits. It stands
				// before an instruction, as far as a pause goes.
				if err == ErrPaused && m.wait != nil {
					if in.freeze() {
						return object.Value{}, ErrPaused
					}
					// A worker something needs goes on waiting.
					err = nil
				}
				continue
			}
			if err = m.unwind(err, 0); err == nil {
				continue
			}
		}
		// An instruction of flat code was left by a completion, or the
		// lists it entered or the call it made were: a loop or a BEGIN
		// the instruction is in may take it.
		if err != nil && m.flat != nil {
			if err = m.flatTakes(err); err == nil {
				continue
			}
		}
		// The activation ended: with its value on its stack, or with the
		// completion err.
		var v object.Value
		if err == nil {
			v = m.pop()
		}
		if v, err = in.leave(m, v, err); m.again {
			// A redo: the block runs again.
			m.again = false
			continue
		}
		in.acts = in.acts[:len(in.acts)-1]
		site := m.site
		in.release(m)
		if len(in.acts) == floor {
			return v, err
		}
		if err != nil {
			if site != nil {
				err = withLine(err, int(site.Line))
			}
			continue
		}
		in.acts[len(in.acts)-1].push(v)
	}
}

// stepping is a builtin that is a stepper, between two of its steps: the
// method, the call it was made with, what its last step kept and what
// its next step is called with. rescue is set when the last step asked
// for the exception its block raises.
type stepping struct {
	builtin *object.Method
	call    *object.Call
	state   []object.Value
	got     object.Got
	rescue  bool
}

// stepAct makes the activation of a call of the builtin meth, which is a
// stepper.
func (in *Interp) stepAct(meth *object.Method, recv object.Value, args []object.Value, kw *object.Map, block object.Value) *machine {
	return &machine{in: in, kind: actStep, stepper: &stepping{
		builtin: meth,
		call:    &object.Call{Self: recv, Args: args, Kw: kw, Block: block},
		got:     object.Got{First: true},
	}}
}

// stepOn runs the next step of the stepper m is an activation of. err is
// the completion its block was left by, nil when the block returned,
// whose value is then on m's stack. It reports called when the step
// pushed the block's activation; otherwise the builtin ended, with the
// value or the completion returned.
func (in *Interp) stepOn(m *machine, err error) (v object.Value, called bool, _ error) {
	st := m.stepper
	block := st.call.Block
	switch {
	case err == nil && len(m.stack) > 0:
		st.got.Value = m.pop()
	case err != nil:
		// A break out of the block ends the builtin with its value, as it
		// ends any call the block was given to.
		if c, ok := err.(*control); ok && c.kind == ctrlBreak && c.target != nil && c.target == blockTarget(block) {
			return c.value, false, nil
		}
		exc, ok := err.(*object.Exception)
		if !ok || !st.rescue {
			return object.Value{}, false, err
		}
		st.got.Raised = exc
	}
	for {
		s, err := st.builtin.Step(in, st.call, st.state, st.got)
		if err != nil {
			// A step that called the block itself, from Go, may come
			// back with the block's break.
			if c, ok := err.(*control); ok && c.kind == ctrlBreak && c.target != nil && c.target == blockTarget(block) {
				return c.value, false, nil
			}
			return object.Value{}, false, err
		}
		if s.Done {
			return s.Value, false, nil
		}
		st.state, st.rescue, st.got = s.State, s.Rescue, object.Got{}
		// A block a script wrote becomes the activation that runs; any
		// other is called here, and the stepper goes on with its value.
		if p, ok := asProc(block); ok && p.written() {
			act, err := in.blockAct(p, s.Yield, nil, object.Value{})
			if err == nil {
				err = in.pushAct(act, nil)
			}
			if err == nil {
				return object.Value{}, true, nil
			}
			return in.stepOn(m, err)
		}
		value, err := in.callBlock(block, s.Yield, nil, object.Value{})
		if err != nil {
			return in.stepOn(m, err)
		}
		st.got.Value = value
	}
}

// leave ends the activation m, which ran to its value v or was left by
// the completion err, and returns what its caller gets: a method takes
// its return and the break out of the block it was given, a block its
// next. A block that is left by a redo is made to run again instead.
func (in *Interp) leave(m *machine, v object.Value, err error) (object.Value, error) {
	switch m.kind {
	case actMethod:
		fr := m.fr
		fr.ended = true
		if m.debugged {
			in.dbg.leave()
		}
		if len(m.env.promises) > 0 {
			in.promises.end(m.env.promises)
		}
		if err == nil {
			if !m.result.IsZero() {
				return m.result, nil
			}
			return v, nil
		}
		if c, ok := err.(*control); ok {
			switch {
			case c.kind == ctrlReturn && c.target == fr:
				return c.value, nil
			case c.kind == ctrlBreak && c.target != nil && c.target == blockTarget(fr.Block):
				return c.value, nil
			}
		}
		return object.Value{}, in.inFile(err, fr)
	case actBlock:
		p := m.proc
		if c, ok := err.(*control); ok && c.kind == ctrlRedo && c.target == nil {
			if err = in.stopped(); err == nil {
				m.stack = m.stack[:0]
				m.flat, m.bases, m.tries = nil, nil, nil
				m.startAt(in.flatOfProc(p.IR), p.IR.proc.GetCode(), true)
				m.again = true
				return object.Value{}, nil
			}
		}
		if m.debugged {
			in.dbg.leave()
		}
		if p.Strict {
			// A lambda's call is a frame of its own, which ends with it.
			m.fr.ended = true
		}
		if err == nil {
			return v, nil
		}
		if c, ok := err.(*control); ok {
			switch {
			case c.kind == ctrlNext && c.target == nil:
				return c.value, nil
			case c.kind == ctrlBreak && c.target == nil:
				if p.Strict {
					return c.value, nil
				}
				c.target = p
				return object.Value{}, c
			case c.kind == ctrlReturn && p.Strict && (c.target == p.Frame || c.target == m.fr):
				// A return in a lambda returns from the lambda: its own
				// frame, or the one it was written in when a block
				// inside it returned.
				return c.value, nil
			}
		}
		return object.Value{}, in.inFile(err, m.fr)
	}
	return v, err
}

// frame makes fr the frame of a call of the method m, whose code is
// code.
func (in *Interp) frame(fr *Frame, m *object.Method, recv object.Value, args []object.Value, kw *object.Map, block object.Value, code *irProc) {
	*fr = Frame{
		Self:    recv,
		Method:  m,
		Owner:   m.Owner,
		Args:    args,
		Kw:      kw,
		Block:   block,
		Lexical: m.Lexical,
		File:    m.File,
	}
	if len(m.Lexical) > 0 {
		fr.DefTarget = m.Lexical[len(m.Lexical)-1]
	} else {
		fr.DefTarget = object.ObjectClass
	}
	if code.proc.GetName() == "initialize" {
		if inst, ok := recv.AsInstance(); ok {
			fr.Building = inst
		} else if exc, ok := recv.AsException(); ok && exc.Building {
			fr.BuildingExc = exc
		}
	}
}

// TailFramesKept is how many activations that made a tail call are kept
// in a row under a debugger, and what cameo --keep-tail-frames keeps.
const TailFramesKept = 256

// KeepTailFrames makes the interpreter keep the activation that makes a
// tail call, up to n of them in a row: the debugger and a dump then show
// the calls that led to where the run is. Beyond n the oldest is dropped,
// so a loop of tail calls still runs in a stack that does not grow. With
// 0, which is how an interpreter starts, a tail call keeps none.
func (in *Interp) KeepTailFrames(n int) { in.tailFrames = n }

// pushCall makes act, the activation of a call the instruction c of m
// makes, the activation that runs. A tail call (FLAG_TAIL) of a method,
// a block or a lambda takes the place of m: its value is the value of m,
// so it goes to the caller of m, and m does not stay on the stack.
func (in *Interp) pushCall(m, act *machine, c *cameopb.Code) error {
	top := len(in.acts) - 1
	if !has(c, cameopb.Flag_FLAG_TAIL) || m.kind == actList || top < 0 || in.acts[top] != m {
		return in.pushAct(act, c)
	}
	if in.tailFrames > 0 {
		// The activation is kept to be shown, and the oldest of a row
		// of them that is too long is dropped.
		m.passing, m.row = true, 1
		if top > 0 && in.acts[top-1].passing {
			m.row = in.acts[top-1].row + 1
		}
		if err := in.pushAct(act, c); err != nil {
			return err
		}
		// A row is cut back to the number kept once it is a quarter
		// longer, so that the cut is not made for every call.
		if spare := in.tailFrames/4 + 1; m.row >= in.tailFrames+spare {
			in.dropActs(top+1-m.row, spare)
		}
		return nil
	}
	in.endsEarly(m, 0)
	act.site = m.site
	if !m.result.IsZero() {
		act.result = m.result
	}
	in.acts[top] = act
	in.release(m)
	return nil
}

// dropActs removes the n activations from index i on, the oldest of a
// row that made tail calls and were kept: the activation above them
// hands its value to the one below.
func (in *Interp) dropActs(i, n int) {
	above := in.acts[i+n]
	debugged := 0
	for _, act := range in.acts[i+n:] {
		if act.debugged {
			debugged++
		}
	}
	// The newest of them first: the debugger's frame of each is then
	// the one right under the frames of what stays.
	for j := i + n - 1; j >= i; j-- {
		m := in.acts[j]
		in.endsEarly(m, debugged)
		above.site = m.site
		if !m.result.IsZero() {
			above.result = m.result
		}
		in.release(m)
	}
	copy(in.acts[i:], in.acts[i+n:])
	clear(in.acts[len(in.acts)-n:])
	in.acts = in.acts[:len(in.acts)-n]
	for j := i; j < len(in.acts) && in.acts[j].passing; j++ {
		in.acts[j].row -= n
	}
}

// endsEarly does what the end of the activation m does besides handing
// on a value (leave), for one that made a tail call and does not stay:
// its frame has ended, so a block written in it has nowhere to return
// to. above is how many frames the debugger has above the one of m.
func (in *Interp) endsEarly(m *machine, above int) {
	if m.debugged {
		in.dbg.drop(above)
	}
	switch m.kind {
	case actMethod:
		m.fr.ended = true
		if len(m.env.promises) > 0 {
			in.promises.end(m.env.promises)
		}
	case actBlock:
		if m.proc.Strict {
			m.fr.ended = true
		}
	}
}

// methodAct makes the activation of a call of the method m, which a
// script defined, with its parameters bound.
func (in *Interp) methodAct(m *object.Method, recv object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (*machine, error) {
	if err := in.stopped(); err != nil {
		return nil, err
	}
	code := m.Body.(*irProc)
	act, ps := in.machine(), in.paramsOf(code)
	if ps.layout == nil {
		panic("eval: a proc without a layout")
	}
	// The frame and the scope of a call that keeps neither are the
	// machine's own, which the next call it runs uses again. Under a
	// debugger, which keeps both while it shows them, they are not.
	bound := &act.own
	if !ps.contained || in.dbg != nil {
		bound = &activation{}
	}
	fr, env := &bound.frame, &bound.env
	in.frame(fr, m, recv, args, kw, block, code)
	act.unit, act.env, act.fr, act.kind = code.unit, env, fr, actMethod
	act.lay(env, ps.layout)
	if err := in.bindParams(ps, args, kw, block, env, fr, true, m.Name, line); err != nil {
		_, err = in.leave(act, object.Value{}, err)
		return nil, err
	}
	act.startAt(in.flatOfProc(code), code.proc.GetCode(), true)
	if in.dbg != nil {
		in.dbg.enter(methodName(m, recv), env, fr)
		act.debugged = true
	}
	return act, nil
}

// blockAct makes the activation of a call of the block or lambda p, which
// a script wrote, with its parameters bound.
func (in *Interp) blockAct(p *Proc, args []object.Value, kw *object.Map, inner object.Value) (*machine, error) {
	if err := in.stopped(); err != nil {
		return nil, err
	}
	fr, ps := p.Frame, in.paramsOf(p.IR)
	// The scope of a call that keeps neither its frame nor its scope
	// is the machine's own, as a method's is (methodAct).
	act := in.machine()
	bound := &act.own
	if !ps.contained || in.dbg != nil {
		bound = &activation{}
	}
	env := &bound.env
	env.parent = p.Env
	if p.Strict {
		// A lambda's call is a frame of its own, where its return stops:
		// it starts and ends with the call, whatever became of the
		// method the lambda was written in.
		bound.frame = *p.Frame
		bound.frame.Lambda, bound.frame.ended = true, false
		fr = &bound.frame
	}
	act.lay(env, ps.layout)
	if err := in.bindParams(ps, args, kw, inner, env, fr, p.Strict, "block", p.line()); err != nil {
		if p.Strict {
			fr.ended = true
		}
		in.release(act)
		return nil, err
	}
	act.unit, act.env, act.fr, act.kind, act.proc = p.IR.unit, env, fr, actBlock, p
	act.startAt(in.flatOfProc(p.IR), p.IR.proc.GetCode(), true)
	if in.dbg != nil {
		in.dbg.enter(blockName(in.dbg, p.Frame), env, fr)
		act.debugged = true
	}
	return act, nil
}

// callsProc reports whether name is a method of Proc that calls the proc
// with its arguments.
func callsProc(name string) bool {
	switch name {
	case "call", "()", "[]", "yield", "===":
		return true
	}
	return false
}

// runs reports whether a call of the method m is an activation the loop
// runs: m is one a script defined. private says whether the call reaches
// a function of Kernel.
func (in *Interp) runs(m *object.Method, private bool) bool {
	return m != nil && m.Body != nil && (!m.Private || private)
}
