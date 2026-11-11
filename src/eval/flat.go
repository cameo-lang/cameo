package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// Flat code: the instructions of a list and of the lists inside it in one
// array, with jumps where the IR has lists. A unit and a dump carry the
// IR as it is; an interpreter lowers a proc to flat code when it first
// calls it, and runs that. A run that pauses turns the position of each
// activation back into cursors (cursorsAt), so a dump does not know of
// flat code. doc/design/ir.md, "Flat code", has the design.
//
// A list is lowered when every instruction in it is one flat code runs:
// IF, WHILE, AND, OR, SAFE and INTERP become jumps, and an instruction
// without lists runs as it does from a cursor (machine.exec). A list
// with any other instruction that holds lists, or with an async call or
// an await, runs from cursors.

// flatOp is what a flat instruction does.
type flatOp uint8

const (
	// fExec runs src as machine.exec does: an instruction without lists.
	fExec flatOp = iota
	// fLoad pushes the local sym, nil when it is unbound; fStore binds
	// the value on top to it.
	fLoad
	fStore
	// fConst pushes val.
	fConst
	// fBinary runs the operator sym on the two values on top.
	fBinary
	// fPop drops the value on top.
	fPop
	// fJump goes to a.
	fJump
	// fIf pops a value and goes to a when it is not truthy: an IF.
	fIf
	// fAnd goes to a when the value on top is not truthy, and drops it
	// otherwise; fOr is the reverse. fSafe goes to a when it is nil.
	fAnd
	fOr
	fSafe
	// fWhile starts a WHILE: it notes the height of the stack as the
	// base of loop level a. fWhileDo is the same for a loop that runs
	// its body first, and checks for a stop as each pass does.
	fWhile
	fWhileDo
	// fTest pops the condition of the loop at level b: not truthy, it
	// pushes nil, forgets the base and goes to a.
	fTest
	// fBack ends a pass of a loop: it checks for a stop and goes to a,
	// the condition.
	fBack
	// fBreak, fNext and fRedo are a break, a next and a redo in the body
	// of the loop at level b: each cuts the stack to the loop's base and
	// goes to a, the instruction after the loop, its condition or its
	// body.
	fBreak
	fNext
	fRedo
	// fInterp pushes the empty string an INTERP starts with; fPart adds
	// the value on top to the string under it.
	fInterp
	fPart
)

// flatIns is one instruction of flat code.
type flatIns struct {
	op flatOp
	// real is set when the instruction is where src starts in its list:
	// a run pauses before such an instruction and no other.
	real bool
	a, b int32
	sym  object.Sym
	val  object.Value
	// src is the instruction of the IR this one is lowered from, in at
	// at index idx.
	src *cameopb.Code
	at  *flatList
	idx int32
}

// flatList is a list of the IR that flat code was lowered from: what a
// cursor of the list says besides its place.
type flatList struct {
	parent *flatList
	code   []*cameopb.Code
	// owner is the instruction the list belongs to, at ownerIdx of
	// parent; nil for the list the activation starts with.
	owner    *cameopb.Code
	ownerIdx int32
	list     int
	phase    phase
	body     bool
	// level is the level of the loop whose condition or body the list
	// is, -1 for any other list.
	level int32
}

// flatCode is a list lowered.
type flatCode struct {
	code []flatIns
}

// lowerer lowers one list and the lists inside it.
type lowerer struct {
	syms *unitNames
	out  []flatIns
	// loops is the loops a break, next or redo at this point belongs to,
	// outermost first; breaks holds, per loop, the fBreak instructions to
	// patch with the loop's end.
	loops  []*flatLoop
	levels int32
	ok     bool
}

type flatLoop struct {
	level      int32
	cond, body int32
	breaks     []int
}

// lower returns code as flat code, or nil when an instruction in it runs
// from cursors alone.
func lower(code []*cameopb.Code, syms *unitNames, body bool) *flatCode {
	if len(code) == 0 {
		return nil
	}
	l := &lowerer{syms: syms, ok: true}
	l.list(&flatList{code: code, body: body, level: -1})
	if !l.ok {
		return nil
	}
	return &flatCode{code: l.out}
}

func (l *lowerer) emit(ins flatIns) int {
	l.out = append(l.out, ins)
	return len(l.out) - 1
}

// here is the index of the next instruction emitted.
func (l *lowerer) here() int32 { return int32(len(l.out)) }

// inner returns list i of c, which is at index idx of at.
func inner(at *flatList, c *cameopb.Code, idx, i int, ph phase, body bool) *flatList {
	return &flatList{parent: at, code: c.Code[i].GetCode(), owner: c, ownerIdx: int32(idx), list: i, phase: ph, body: body, level: -1}
}

// list lowers the instructions of at.
func (l *lowerer) list(at *flatList) {
	for idx, c := range at.code {
		if !l.ok {
			return
		}
		ins := flatIns{real: true, src: c, at: at, idx: int32(idx)}
		switch c.Op {
		case cameopb.Op_IF:
			if len(c.Code) != 2 {
				l.ok = false
				return
			}
			ins.op = fIf
			test := l.emit(ins)
			l.list(inner(at, c, idx, 0, phaseValue, true))
			end := l.emit(flatIns{op: fJump})
			l.out[test].a = l.here()
			l.list(inner(at, c, idx, 1, phaseValue, true))
			l.out[end].a = l.here()
		case cameopb.Op_AND, cameopb.Op_OR, cameopb.Op_SAFE:
			ins.op = map[cameopb.Op]flatOp{cameopb.Op_AND: fAnd, cameopb.Op_OR: fOr, cameopb.Op_SAFE: fSafe}[c.Op]
			test := l.emit(ins)
			l.list(inner(at, c, idx, 0, phaseValue, false))
			l.out[test].a = l.here()
		case cameopb.Op_WHILE:
			l.while(at, c, idx, ins)
		case cameopb.Op_INTERP:
			ins.op = fInterp
			l.emit(ins)
			for i := range c.Code {
				l.list(inner(at, c, idx, i, phasePart, false))
				l.emit(flatIns{op: fPart, src: c})
			}
		case cameopb.Op_BREAK, cameopb.Op_NEXT, cameopb.Op_REDO:
			// In the body of a loop of this list they are jumps; anywhere
			// else they leave the activation, as exec has them do.
			if len(l.loops) == 0 {
				l.emit(ins)
				break
			}
			loop := l.loops[len(l.loops)-1]
			ins.b = loop.level
			switch c.Op {
			case cameopb.Op_BREAK:
				ins.op = fBreak
				loop.breaks = append(loop.breaks, l.emit(ins))
			case cameopb.Op_NEXT:
				ins.op, ins.a = fNext, loop.cond
				l.emit(ins)
			default:
				ins.op, ins.a = fRedo, loop.body
				l.emit(ins)
			}
		case cameopb.Op_ASYNC, cameopb.Op_AWAIT, cameopb.Op_AWAIT_CALL, cameopb.Op_AWAIT_ALL:
			// An activation that awaits waits in the loop of run.
			l.ok = false
		case cameopb.Op_LOAD:
			ins.op, ins.sym = fLoad, l.syms.syms[c.Args[0].GetName()]
			l.emit(ins)
		case cameopb.Op_STORE:
			if !has(c, cameopb.Flag_FLAG_FORWARD) {
				ins.op, ins.sym = fStore, l.syms.syms[c.Args[0].GetName()]
			}
			l.emit(ins)
		case cameopb.Op_CONST:
			if v, ok := plainConstant(c); ok {
				ins.op, ins.val = fConst, v
			}
			l.emit(ins)
		case cameopb.Op_DROP, cameopb.Op_POP:
			ins.op = fPop
			l.emit(ins)
		default:
			switch {
			case c.Op >= cameopb.Op_BINARY_PLUS && c.Op <= cameopb.Op_BINARY_NE:
				ins.op, ins.sym = fBinary, object.SymAdd+object.Sym(c.Op-cameopb.Op_BINARY_PLUS)
			case len(c.Code) > 0:
				// An instruction with lists that flat code has no jumps
				// for: BEGIN, CASE, CASE_IN, a REGEXP with parts, defined?.
				l.ok = false
			}
			l.emit(ins)
		}
	}
}

// while lowers a WHILE at index idx of at: the condition, a test that
// leaves the loop, the body, and a jump back. A loop that runs its body
// first starts at the body.
func (l *lowerer) while(at *flatList, c *cameopb.Code, idx int, ins flatIns) {
	loop := &flatLoop{level: l.levels}
	l.levels++
	ins.op, ins.a = fWhile, loop.level
	if has(c, cameopb.Flag_FLAG_DO_WHILE) {
		ins.op = fWhileDo
	}
	l.emit(ins)
	enter := -1
	if ins.op == fWhileDo {
		enter = l.emit(flatIns{op: fJump})
	}
	// A break, next or redo in the condition is not this loop's.
	loop.cond = l.here()
	cond := inner(at, c, idx, 0, phaseCond, false)
	cond.level = loop.level
	l.list(cond)
	test := l.emit(flatIns{op: fTest, b: loop.level})
	loop.body = l.here()
	if enter >= 0 {
		l.out[enter].a = loop.body
	}
	body := inner(at, c, idx, 1, phaseBody, true)
	body.level = loop.level
	l.loops = append(l.loops, loop)
	l.list(body)
	l.loops = l.loops[:len(l.loops)-1]
	l.emit(flatIns{op: fPop})
	l.emit(flatIns{op: fBack, a: loop.cond})
	end := l.here()
	l.out[test].a = end
	for _, at := range loop.breaks {
		l.out[at].a = end
	}
	l.levels--
}

// plainConstant returns the value of a CONST that is the same value each
// time it runs, and false for a big integer, which is made once per
// program by the instruction itself.
func plainConstant(c *cameopb.Code) (object.Value, bool) {
	switch k := c.Args[0].GetConstant().GetKind().(type) {
	case nil:
		return object.Nil, true
	case *cameopb.Constant_BoolValue:
		return object.Bool(k.BoolValue), true
	case *cameopb.Constant_IntValue:
		return object.Int(k.IntValue), true
	case *cameopb.Constant_FloatValue:
		return object.Float(k.FloatValue), true
	case *cameopb.Constant_StringValue:
		return object.String(k.StringValue), true
	}
	return object.Value{}, false
}

// runFlat runs the flat code of m from where it is until the activation
// ends, a call became the activation that runs, or a pause is asked for
// and the next instruction is one a run pauses before. It returns the
// completion that leaves the activation.
func (m *machine) runFlat() error {
	in, code := m.in, m.flat.code
	yield, started := in.pausing.flag.Load(), false
	for m.pc < len(code) {
		ins := &code[m.pc]
		if yield && started && ins.real {
			return nil
		}
		started = true
		m.pc++
		switch ins.op {
		case fLoad:
			if v, ok := m.env.GetSym(ins.sym); ok {
				m.stack = append(m.stack, v)
			} else {
				m.stack = append(m.stack, object.Nil)
			}
		case fStore:
			m.env.SetSym(ins.sym, m.stack[len(m.stack)-1])
		case fConst:
			m.stack = append(m.stack, ins.val)
		case fBinary:
			n := len(m.stack)
			if v := binary(ins.sym, m.stack[n-2], m.stack[n-1]); !v.IsZero() {
				m.stack[n-2] = v
				m.stack = m.stack[:n-1]
				continue
			}
			if err := m.binary(ins.src); err != nil {
				return err
			}
			if in.acts[len(in.acts)-1] != m {
				return nil
			}
			yield = in.pausing.flag.Load()
		case fPop:
			m.stack = m.stack[:len(m.stack)-1]
		case fJump:
			m.pc = int(ins.a)
		case fIf:
			if !m.pop().Truthy() {
				m.pc = int(ins.a)
			}
		case fAnd:
			if m.top().Truthy() {
				m.pop()
			} else {
				m.pc = int(ins.a)
			}
		case fOr:
			if !m.top().Truthy() {
				m.pop()
			} else {
				m.pc = int(ins.a)
			}
		case fSafe:
			if m.top().IsNil() {
				m.pc = int(ins.a)
			}
		case fWhile, fWhileDo:
			m.bases = append(m.bases[:ins.a], len(m.stack))
			if err := in.stopped(); err != nil {
				return err
			}
		case fTest:
			if !m.pop().Truthy() {
				m.bases = m.bases[:ins.b]
				m.stack = append(m.stack, object.Nil)
				m.pc = int(ins.a)
			}
		case fBack:
			if err := in.stopped(); err != nil {
				return err
			}
			m.pc = int(ins.a)
			yield = in.pausing.flag.Load()
		case fBreak:
			v := m.pop()
			m.stack = append(m.stack[:m.bases[ins.b]], v)
			m.bases = m.bases[:ins.b]
			m.pc = int(ins.a)
		case fNext:
			m.stack = m.stack[:m.bases[ins.b]]
			m.bases = m.bases[:ins.b+1]
			if err := in.stopped(); err != nil {
				return err
			}
			m.pc = int(ins.a)
			yield = in.pausing.flag.Load()
		case fRedo:
			m.stack = m.stack[:m.bases[ins.b]]
			m.bases = m.bases[:ins.b+1]
			if err := in.stopped(); err != nil {
				return err
			}
			m.pc = int(ins.a)
			yield = in.pausing.flag.Load()
		case fInterp:
			m.stack = append(m.stack, object.String(""))
		case fPart:
			if err := m.interpAdd(ins.src); err != nil {
				return err
			}
			yield = in.pausing.flag.Load()
		default:
			if err := m.exec(ins.src); err != nil {
				return err
			}
			if in.acts[len(in.acts)-1] != m {
				return nil
			}
			yield = in.pausing.flag.Load()
		}
	}
	return nil
}

// atReal reports whether the next flat instruction of m is one a run
// pauses before.
func (m *machine) atReal() bool {
	return m.pc < len(m.flat.code) && m.flat.code[m.pc].real
}

// cursorsAt turns the place of m in its flat code into cursors, and m
// into an activation that runs from them: what a dump writes, and what
// a run that goes on after a pause continues. called is set for an
// activation that called the one above it: its place is after the
// instruction that called. Any other is before its next instruction,
// which is one a run pauses before.
func (m *machine) cursorsAt(called bool) {
	at := m.pc
	if called {
		at--
	}
	ins := &m.flat.code[at]
	next := ins.idx
	if called {
		next++
	}
	var lists []*flatList
	for l := ins.at; l != nil; l = l.parent {
		lists = append(lists, l)
	}
	m.cursors = m.cursors[:0]
	for i := len(lists) - 1; i >= 0; i-- {
		l := lists[i]
		cur := cursor{code: l.code, owner: l.owner, list: l.list, phase: l.phase, base: len(m.stack)}
		if l.level >= 0 {
			cur.base = m.bases[l.level]
		}
		// A cursor of a list around this one is after the instruction
		// the inner list belongs to.
		cur.next = int(next)
		if i > 0 {
			cur.next = int(lists[i-1].ownerIdx) + 1
		}
		switch {
		case cur.next > 0:
			cur.statement = l.code[cur.next-1].Op == cameopb.Op_DROP
		case l.owner == nil:
			cur.statement = m.flatBody
		default:
			cur.statement = l.body
		}
		m.cursors = append(m.cursors, cur)
	}
	m.flat, m.pc, m.bases = nil, 0, nil
}
