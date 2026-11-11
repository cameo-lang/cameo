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
		if len(m.cursors) > 0 {
			if err == nil {
				err = m.step()
				continue
			}
			if err = m.unwind(err, 0); err == nil {
				continue
			}
		}
		// The activation ended: with its value on its stack, or with the
		// completion err.
		var v object.Value
		if err == nil {
			v = m.pop()
		}
		if v, err = in.leave(m, v, err); len(m.cursors) > 0 {
			// A redo: the block runs again.
			continue
		}
		in.acts = in.acts[:len(in.acts)-1]
		if len(in.acts) == floor {
			return v, err
		}
		if err != nil {
			err = withLine(err, int(m.site.Line))
			continue
		}
		in.acts[len(in.acts)-1].push(v)
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
				m.cursors = append(m.cursors, cursor{code: p.IR.proc.GetCode(), statement: true})
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

// methodAct makes the activation of a call of the method m, which a
// script defined, with its parameters bound.
func (in *Interp) methodAct(m *object.Method, recv object.Value, args []object.Value, kw *object.Map, block object.Value, line int) (*machine, error) {
	if err := in.stopped(); err != nil {
		return nil, err
	}
	code := m.Body.(*irProc)
	bound := &activation{frame: Frame{
		Self:    recv,
		Method:  m,
		Owner:   m.Owner,
		Args:    args,
		Kw:      kw,
		Block:   block,
		Lexical: m.Lexical,
		File:    m.File,
	}}
	fr, env := &bound.frame, &bound.env
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
	act := &machine{in: in, unit: code.unit, env: env, fr: fr, kind: actMethod}
	if err := in.bindParams(in.code.irParams(code), args, kw, block, env, fr, true, m.Name, line); err != nil {
		_, err = in.leave(act, object.Value{}, err)
		return nil, err
	}
	act.cursors = append(act.cursors, cursor{code: code.proc.GetCode(), statement: true})
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
	env := NewEnv(p.Env)
	fr := p.Frame
	if p.Strict {
		// A lambda's call is a frame of its own, where its return stops:
		// it starts and ends with the call, whatever became of the
		// method the lambda was written in.
		copy := *p.Frame
		copy.Lambda, copy.ended = true, false
		fr = &copy
	}
	if err := in.bindParams(in.code.irParams(p.IR), args, kw, inner, env, fr, p.Strict, "block", p.line()); err != nil {
		if p.Strict {
			fr.ended = true
		}
		return nil, err
	}
	act := &machine{in: in, unit: p.IR.unit, env: env, fr: fr, kind: actBlock, proc: p}
	act.cursors = append(act.cursors, cursor{code: p.IR.proc.GetCode(), statement: true})
	if in.dbg != nil {
		in.dbg.enter(blockName(in.dbg, p.Frame), env, fr)
		act.debugged = true
	}
	return act, nil
}

// runs reports whether a call of the method m is an activation the loop
// runs: m is one a script defined, and not one `cameo build` compiled
// that runs as Go. private says whether the call reaches a function of
// Kernel.
func (in *Interp) runs(m *object.Method, private bool) bool {
	return m != nil && m.Body != nil && (m.Native == nil || in.interpreted > 0) && (!m.Private || private)
}
