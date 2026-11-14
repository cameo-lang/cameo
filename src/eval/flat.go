package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/str"
)

// Flat code: the instructions of a list and of the lists inside it in one
// array, with jumps where the IR has lists. A unit and a dump carry the
// IR as it is; an interpreter lowers a proc to flat code when it first
// calls it, and runs that. A run that pauses turns the position of each
// activation back into cursors (cursorsAt), so a dump does not know of
// flat code. doc/design/ir.md, "Flat code", has the design.
//
// IF, WHILE, CASE, BEGIN, AND, OR, SAFE and INTERP become jumps, and an
// instruction without lists runs as it does from a cursor (machine.exec).
// Any other instruction that holds lists (CASE_IN, a REGEXP with parts,
// defined? of an expression) runs from cursors inside the flat
// code: it enters its lists as it always did, the activation runs them
// from cursors until none is left, and goes on in its flat code. A list
// with an async call or an await anywhere in it runs from cursors
// altogether.

// flatOp is what a flat instruction does.
type flatOp uint8

const (
	// fExec runs src as machine.exec does: an instruction without lists.
	fExec flatOp = iota
	// fLoad pushes the local sym, nil when it is unbound; fStore binds
	// the value on top to it.
	fLoad
	fStore
	// fLoadSlot and fStoreSlot are fLoad and fStore of the variable in
	// slot a of the layout the code was lowered for, when the scope has
	// that layout; they go by the name otherwise.
	fLoadSlot
	fStoreSlot
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
	// fWhen tests the conditions of a when of a CASE, which are on top
	// as a list: with one that fits it drops them and the subject and
	// goes on in the body; with none it drops them and goes to a, the
	// next when. fElse drops the subject before the else of a CASE.
	fWhen
	fElse
	// fTree runs src, which holds lists, as machine.exec does: it enters
	// a list, and the activation runs from cursors until none is left.
	fTree
	// fBegin starts a BEGIN: it notes the height of the stack as the base
	// of the BEGIN at level a. Its body follows, then its else, its
	// rescue clauses and its ensure.
	fBegin
	// fRescueTest tests the classes of rescue clause b of the BEGIN at
	// level a, which are on top as a list, against the exception the
	// BEGIN is trying its clauses for: it goes on in the body of the
	// clause, or tries the clauses after it.
	fRescueTest
	// fRescued ends a rescue clause of the BEGIN at level a: the
	// exception the clause around was handling is the one handled again.
	fRescued
	// fEnsureEnd ends the ensure of the BEGIN at level a: it drops the
	// value of the clause, and what the clause interrupted goes on.
	fEnsureEnd
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
	// scope is the innermost loop body or list of a BEGIN the
	// instruction is in, an index into the scopes of the flat code, -1
	// for none: what takes a completion the instruction is left by.
	scope int32
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
	// is, -1 for any other list; try is the level of the BEGIN the list
	// is one of, -1 for any other.
	level int32
	try   int32
}

// flatCode is a list lowered, and the scopes its instructions are in.
type flatCode struct {
	code   []flatIns
	scopes []flatScope
	// layout is the layout of the scope the code was lowered for, nil
	// when it reads every variable by name.
	layout *layout
}

// flatScope is a place a completion goes through on its way out of flat
// code: the body of a loop, which takes a break, a next and a redo, or a
// list of a BEGIN, which rescues, retries and runs its ensure.
type flatScope struct {
	parent int32
	// loop is set for the body of a loop.
	loop *flatLoop
	// begin is set for a list of a BEGIN, the one phase says.
	begin *flatBegin
	phase phase
}

// flatBegin is a BEGIN lowered: where its lists start in the flat code.
type flatBegin struct {
	src   *cameopb.Code
	level int32
	// body and ensure are where the body and the ensure start; classes
	// and bodies are, per rescue clause, where its classes and its body
	// start. A clause without classes has -1.
	body, ensure    int32
	classes, bodies []int32
}

// try is what an activation keeps of a BEGIN it is inside: the height of
// the value stack when it started, the exception its rescue clauses are
// tried for or the one a clause is handling, the exception the clause
// around was handling, and what its ensure interrupted. A cursor of a
// list of a BEGIN keeps the same.
type try struct {
	base       int
	exc, outer *object.Exception
	pending    error
}

// lowerer lowers one list and the lists inside it.
type lowerer struct {
	syms *unitNames
	// lay is the layout of the scope the code runs in, nil for none;
	// top is that scope when its layout is its own, which the lowerer
	// adds the names of the code to.
	lay *layout
	top *Env
	out []flatIns
	// scopes is the scopes of the code so far, and scope the one the
	// next instruction is in, -1 for none.
	scopes []flatScope
	scope  int32
	// levels and tries are how many loops and BEGINs the next
	// instruction is inside.
	levels, tries int32
	ok            bool
}

type flatLoop struct {
	level           int32
	cond, body, end int32
	breaks          []int
}

// lower returns code as flat code, or nil when an instruction in it runs
// from cursors alone.
func lower(code []*cameopb.Code, syms *unitNames, body bool, lay *layout, top *Env) *flatCode {
	if len(code) == 0 {
		return nil
	}
	l := &lowerer{syms: syms, lay: lay, top: top, ok: true, scope: -1}
	l.list(&flatList{code: code, body: body, level: -1, try: -1})
	if !l.ok {
		return nil
	}
	if top != nil {
		lay = top.layout
	}
	return &flatCode{code: l.out, scopes: l.scopes, layout: lay}
}

func (l *lowerer) emit(ins flatIns) int {
	ins.scope = l.scope
	l.out = append(l.out, ins)
	return len(l.out) - 1
}

// in lowers at inside a new scope, sc, of the scope the lowerer is in.
func (l *lowerer) in(sc flatScope, at *flatList) {
	outer := l.scope
	sc.parent = outer
	l.scopes = append(l.scopes, sc)
	l.scope = int32(len(l.scopes) - 1)
	l.list(at)
	l.scope = outer
}

// jumpLoop returns the loop a break, next or redo at this point jumps
// to: the one whose body it is in with no list of a BEGIN between them,
// which has an ensure to run or a clause to end first.
func (l *lowerer) jumpLoop() *flatLoop {
	if l.scope < 0 {
		return nil
	}
	return l.scopes[l.scope].loop
}

// slot is the slot of name in the layout the code is lowered for, -1
// when it has none.
func (l *lowerer) slot(name object.Sym) int32 {
	if l.top != nil {
		return int32(l.top.reserve(name))
	}
	if l.lay == nil {
		return -1
	}
	return int32(l.lay.slot(name))
}

// here is the index of the next instruction emitted.
func (l *lowerer) here() int32 { return int32(len(l.out)) }

// inner returns list i of c, which is at index idx of at.
func inner(at *flatList, c *cameopb.Code, idx, i int, ph phase, body bool) *flatList {
	return &flatList{parent: at, code: c.Code[i].GetCode(), owner: c, ownerIdx: int32(idx), list: i, phase: ph, body: body, level: -1, try: -1}
}

// list lowers the instructions of at.
func (l *lowerer) list(at *flatList) {
	// skip is how many instructions after this one are a part of it
	// already: the entries of a map that is one value.
	skip := 0
	for idx, c := range at.code {
		if !l.ok {
			return
		}
		if skip > 0 {
			skip--
			continue
		}
		ins := flatIns{real: true, src: c, at: at, idx: int32(idx)}
		switch c.Op {
		case cameopb.Op_SYM:
			ins.op, ins.val = fConst, object.Symbol(l.syms.syms[c.Args[0].GetName()].String())
			l.emit(ins)
		case cameopb.Op_LIST:
			// A list of literals (FLAG_STATIC) is one value, made here:
			// values do not change, so every run of the code has the
			// same list. It takes the place of its first element, where
			// a jump to the list's start arrives.
			if n := int(c.Args[0].GetInt()); has(c, cameopb.Flag_FLAG_STATIC) && l.literals(at, idx, n) {
				elems := make([]object.Value, n)
				for i, e := range l.out[len(l.out)-n:] {
					elems[i] = e.val
				}
				if n > 0 {
					ins = l.out[len(l.out)-n]
					l.out = l.out[:len(l.out)-n]
				}
				ins.op, ins.val = fConst, object.NewList(elems...)
			}
			l.emit(ins)
		case cameopb.Op_MAP:
			// A map of literals likewise, with its entries after it.
			if m, n := l.staticMap(at, idx); m != nil {
				ins.op, ins.val = fConst, object.MapValue(m)
				skip = n
			}
			l.emit(ins)
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
		case cameopb.Op_CASE:
			l.caseWhen(at, c, idx, ins)
		case cameopb.Op_BEGIN:
			if len(c.Code) < 3 || len(c.Code)%2 == 0 || awaits(c) {
				l.ok = false
				return
			}
			l.begin(at, c, idx, ins)
		case cameopb.Op_INTERP:
			ins.op = fInterp
			l.emit(ins)
			for i := range c.Code {
				l.list(inner(at, c, idx, i, phasePart, false))
				// b is set for the last part.
				part := flatIns{op: fPart, src: c}
				if i+1 == len(c.Code) {
					part.b = 1
				}
				l.emit(part)
			}
		case cameopb.Op_BREAK, cameopb.Op_NEXT, cameopb.Op_REDO:
			// In the body of a loop of this list they are jumps; anywhere
			// else they are completions, as exec has them be, which the
			// scopes around take or the activation is left by.
			loop := l.jumpLoop()
			if loop == nil {
				l.emit(ins)
				break
			}
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
			if slot := l.slot(ins.sym); slot >= 0 {
				ins.op, ins.a = fLoadSlot, slot
			}
			l.emit(ins)
		case cameopb.Op_STORE:
			if !has(c, cameopb.Flag_FLAG_FORWARD) {
				ins.op, ins.sym = fStore, l.syms.syms[c.Args[0].GetName()]
				if slot := l.slot(ins.sym); slot >= 0 {
					ins.op, ins.a = fStoreSlot, slot
				}
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
				// for: CASE_IN, a REGEXP with parts, defined?.
				if awaits(c) {
					l.ok = false
					return
				}
				ins.op = fTree
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
	l.in(flatScope{loop: loop}, body)
	l.emit(flatIns{op: fPop})
	l.emit(flatIns{op: fBack, a: loop.cond})
	end := l.here()
	l.out[test].a, loop.end = end, end
	for _, at := range loop.breaks {
		l.out[at].a = end
	}
	l.levels--
}

// caseWhen lowers a CASE at index idx of at: per when its conditions, a
// test and its body, then the else. The subject, when the CASE has one,
// stays on the stack until a body or the else is entered.
func (l *lowerer) caseWhen(at *flatList, c *cameopb.Code, idx int, ins flatIns) {
	last := len(c.Code) - 1
	var ends []int
	real := true
	for i := 0; i < last; i += 2 {
		start := l.here()
		l.list(inner(at, c, idx, i, phaseConds, false))
		if real && l.here() > start {
			// The CASE starts where its first condition does.
			real = false
		}
		test := l.emit(flatIns{op: fWhen, src: c})
		l.list(inner(at, c, idx, i+1, phaseValue, true))
		ends = append(ends, l.emit(flatIns{op: fJump}))
		l.out[test].a = l.here()
	}
	// A CASE without a condition to start at starts at its else.
	ins.op, ins.real = fElse, real
	l.emit(ins)
	l.list(inner(at, c, idx, last, phaseValue, true))
	for _, end := range ends {
		l.out[end].a = l.here()
	}
}

// begin lowers a BEGIN at index idx of at. Its lists are the body, the
// else and the ensure, then the classes and the body of each rescue
// clause; they are laid out as the body, the else, the clauses and the
// ensure, which every way out of the others goes through. A rescue
// clause is reached by the scope of the body, which an exception leaves
// the body through (flatTakes).
func (l *lowerer) begin(at *flatList, c *cameopb.Code, idx int, ins flatIns) {
	bg := &flatBegin{src: c, level: l.tries}
	l.tries++
	sub := func(i int, ph phase, body bool) {
		list := inner(at, c, idx, i, ph, body)
		list.try = bg.level
		l.in(flatScope{begin: bg, phase: ph}, list)
	}
	ins.op, ins.a = fBegin, bg.level
	l.emit(ins)
	bg.body = l.here()
	sub(0, phaseBody, true)
	if has(c, cameopb.Flag_FLAG_ELSE) {
		l.emit(flatIns{op: fPop})
		sub(1, phaseElse, true)
	}
	var ends []int
	for i := 0; 3+2*i < len(c.Code); i++ {
		ends = append(ends, l.emit(flatIns{op: fJump}))
		bg.classes = append(bg.classes, -1)
		if len(c.Code[3+2*i].GetCode()) > 0 {
			bg.classes[i] = l.here()
			sub(3+2*i, phaseClasses, false)
			// The test is in the scope of the classes: a completion it
			// is left by leaves the BEGIN as one out of them does.
			l.scopes = append(l.scopes, flatScope{parent: l.scope, begin: bg, phase: phaseClasses})
			l.out[l.emit(flatIns{op: fRescueTest, a: bg.level, b: int32(i), src: c})].scope = int32(len(l.scopes) - 1)
		}
		bg.bodies = append(bg.bodies, l.here())
		sub(4+2*i, phaseRescue, true)
		l.emit(flatIns{op: fRescued, a: bg.level})
	}
	for _, end := range ends {
		l.out[end].a = l.here()
	}
	bg.ensure = -1
	if has(c, cameopb.Flag_FLAG_ENSURE) {
		bg.ensure = l.here()
		sub(2, phaseEnsure, true)
		l.emit(flatIns{op: fEnsureEnd, a: bg.level})
	}
	l.tries--
}

// awaits returns whether c or an instruction in its lists is an async
// call or an await.
func awaits(c *cameopb.Code) bool {
	switch c.Op {
	case cameopb.Op_ASYNC, cameopb.Op_AWAIT, cameopb.Op_AWAIT_CALL, cameopb.Op_AWAIT_ALL:
		return true
	}
	for _, seq := range c.Code {
		for _, inner := range seq.GetCode() {
			if awaits(inner) {
				return true
			}
		}
	}
	return false
}

// literals returns whether the n instructions lowered last are the
// constants of the n instructions before idx in at: the elements of a
// list of literals.
func (l *lowerer) literals(at *flatList, idx, n int) bool {
	if n > idx || n > len(l.out) {
		return false
	}
	for i, e := range l.out[len(l.out)-n:] {
		if e.op != fConst || e.at != at || int(e.idx) != idx-n+i {
			return false
		}
	}
	return true
}

// staticMap returns the map a MAP of literals at idx in at makes
// (FLAG_STATIC), and how many instructions after the MAP are its
// entries: a value, a key and a MAP_SET for each. It returns nil for
// any other MAP.
func (l *lowerer) staticMap(at *flatList, idx int) (*object.Map, int) {
	c := at.code[idx]
	if !has(c, cameopb.Flag_FLAG_STATIC) || len(c.Args) == 0 {
		return nil, 0
	}
	n := int(c.Args[0].GetInt())
	entries := at.code[idx+1:]
	if len(entries) < 3*n {
		return nil, 0
	}
	m := object.NewMapSized(n)
	for i := 0; i < n; i++ {
		v, ok := l.literal(entries[3*i])
		k, kok := l.literal(entries[3*i+1])
		if !ok || !kok || entries[3*i+2].Op != cameopb.Op_MAP_SET {
			return nil, 0
		}
		m.Add(k, v)
	}
	return m, 3 * n
}

// literal returns the value of a CONST or a SYM.
func (l *lowerer) literal(c *cameopb.Code) (object.Value, bool) {
	switch c.Op {
	case cameopb.Op_CONST:
		return plainConstant(c)
	case cameopb.Op_SYM:
		return object.Symbol(l.syms.syms[c.Args[0].GetName()].String()), true
	}
	return object.Value{}, false
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
		return object.NewString(k.StringValue, str.UTF8), true
	}
	return object.Value{}, false
}

// runFlat runs the flat code of m from where it is until the activation
// ends, a call became the activation that runs, or a pause is asked for
// and the next instruction is one a run pauses before. It returns the
// completion that leaves the activation.
func (m *machine) runFlat() error {
	in, flat := m.in, m.flat
	code, lay := flat.code, flat.layout
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
		case fLoadSlot:
			e := m.env
			if e.layout == lay {
				if v := e.slots[ins.a]; !v.IsZero() {
					m.stack = append(m.stack, v)
					continue
				}
			}
			m.stack = append(m.stack, m.loadSlot(ins))
		case fStoreSlot:
			// A scope inside a captured one, or inside none, binds its
			// own variables alone.
			if e := m.env; e.layout == lay && (e.parent == nil || e.parent.captured) {
				e.slots[ins.a] = m.stack[len(m.stack)-1]
			} else {
				e.SetSym(ins.sym, m.stack[len(m.stack)-1])
			}
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
			m.stack = append(m.stack, object.NewString("", str.ASCII))
		case fPart:
			if err := m.interpAdd(ins.src, ins.b == 1); err != nil {
				return err
			}
			yield = in.pausing.flag.Load()
		case fWhen:
			hit, err := m.caseHit(ins.src)
			if err != nil {
				return err
			}
			if !hit {
				m.pc = int(ins.a)
			}
			yield = in.pausing.flag.Load()
		case fElse:
			if has(ins.src, cameopb.Flag_FLAG_SUBJECT) {
				m.pop()
			}
		case fBegin:
			m.tries = append(m.tries[:ins.a], try{base: len(m.stack)})
		case fRescueTest:
			if err := m.rescueTest(ins); err != nil {
				return err
			}
		case fRescued:
			in.rescuing = m.tries[ins.a].outer
		case fEnsureEnd:
			m.stack = m.stack[:len(m.stack)-1]
			if err := m.tries[ins.a].pending; err != nil {
				m.tries[ins.a].pending = nil
				return err
			}
		case fTree:
			// The instruction entered a list, or did its work without
			// one; with a list entered, run goes on from the cursors.
			if err := m.exec(ins.src); err != nil {
				return err
			}
			if len(m.cursors) > 0 || in.acts[len(in.acts)-1] != m {
				return nil
			}
			yield = in.pausing.flag.Load()
		default:
			if err := m.exec(ins.src); err != nil {
				return err
			}
			// A call became the activation that runs, or a tail call
			// made this one run other code (tailLoop).
			if in.acts[len(in.acts)-1] != m || m.flat != flat {
				return nil
			}
			yield = in.pausing.flag.Load()
		}
	}
	return nil
}

// loadSlot reads the variable of an fLoadSlot the scope of m does not
// hold in the slot: from the scope a block captured, which has the
// layout of the block too, or by name from the scopes around; nil when
// it is unbound.
func (m *machine) loadSlot(ins *flatIns) object.Value {
	e := m.env
	if e.layout == m.flat.layout {
		// The scope does not hold the name: its slot is empty.
		if e = e.parent; e != nil && e.layout == m.flat.layout {
			if v := e.slots[ins.a]; !v.IsZero() {
				return v
			}
			e = e.parent
		}
	}
	if e != nil {
		if v, ok := e.GetSym(ins.sym); ok {
			return v
		}
	}
	return object.Nil
}

// flatTakes finds the scope of the flat code of m that takes the
// completion err, which the last instruction was left by, or the lists
// it entered or the call it made were: the body of a loop takes a break,
// a next and a redo, and a list of a BEGIN goes on in a rescue clause, in
// the body again or in the ensure. It returns nil when one took it, and
// m goes on where that one says; otherwise the completion, which a clause
// it went through may have changed.
func (m *machine) flatTakes(err error) error {
	if m.pc == 0 {
		return err
	}
	for s := m.flat.code[m.pc-1].scope; s >= 0 && err != nil; {
		sc := &m.flat.scopes[s]
		s = sc.parent
		if sc.begin != nil {
			err = m.beginTakes(sc.begin, sc.phase, err)
			continue
		}
		ctl, ok := err.(*control)
		if !ok || ctl.target != nil {
			continue
		}
		loop := sc.loop
		switch ctl.kind {
		case ctrlBreak:
			m.stack = append(m.stack[:m.bases[loop.level]], ctl.value)
			m.bases = m.bases[:loop.level]
			m.pc = int(loop.end)
			return nil
		case ctrlNext, ctrlRedo:
			m.stack = m.stack[:m.bases[loop.level]]
			m.bases = m.bases[:loop.level+1]
			if err = m.in.stopped(); err != nil {
				continue
			}
			m.pc = int(loop.cond)
			if ctl.kind == ctrlRedo {
				m.pc = int(loop.body)
			}
			return nil
		}
	}
	return err
}

// beginTakes continues the BEGIN bg after the completion err left its
// list of the given phase, as beginLeft does for a cursor: a rescue
// clause may take an exception out of the body, the body runs again for
// a retry out of a clause, and the ensure runs before the completion
// goes on. It returns nil when the BEGIN took the completion or its
// ensure runs first.
func (m *machine) beginTakes(bg *flatBegin, ph phase, err error) error {
	t := &m.tries[bg.level]
	switch ph {
	case phaseBody:
		if exc, ok := err.(*object.Exception); ok {
			m.stack = m.stack[:t.base]
			t.exc = exc
			return m.rescueFlat(bg, 0)
		}
	case phaseRescue:
		m.in.rescuing = t.outer
		if ctl, ok := err.(*control); ok && ctl.kind == ctrlRetry {
			m.stack = m.stack[:t.base]
			t.exc, t.outer = nil, nil
			m.pc = int(bg.body)
			return nil
		}
		err = caused(err, t.exc)
	case phaseEnsure:
		// The ensure did not end: what it interrupted is gone.
		t.pending = nil
		return err
	}
	return m.beginLeaves(bg, err)
}

// beginLeaves leaves the BEGIN bg with a completion none of its clauses
// took: through its ensure, when it has one.
func (m *machine) beginLeaves(bg *flatBegin, err error) error {
	if bg.ensure < 0 {
		return err
	}
	t := &m.tries[bg.level]
	m.stack = m.stack[:t.base]
	t.exc, t.outer, t.pending = nil, nil, err
	m.pc = int(bg.ensure)
	return nil
}

// rescueFlat tries the rescue clauses of bg from clause i on for the
// exception the BEGIN keeps: it goes on in the body of the first that
// fits, or in the classes of the first that names some. With no clause
// left the exception leaves the BEGIN.
func (m *machine) rescueFlat(bg *flatBegin, i int) error {
	t := &m.tries[bg.level]
	for ; i < len(bg.bodies); i++ {
		if bg.classes[i] >= 0 {
			m.pc = int(bg.classes[i])
			return nil
		}
		if t.exc.Cls.IsA(object.StandardError) {
			m.rescued(bg, i)
			return nil
		}
	}
	return m.beginLeaves(bg, t.exc)
}

// rescued goes on in the body of rescue clause i of bg, for the
// exception the BEGIN keeps.
func (m *machine) rescued(bg *flatBegin, i int) {
	in, t := m.in, &m.tries[bg.level]
	if in.rescuing != nil {
		// Raised inside another rescue clause and rescued before it
		// ends: it takes its cause where it is caught.
		t.exc = caused(t.exc, in.rescuing).(*object.Exception)
	}
	if v := m.name(bg.src, i); v != "" {
		m.env.Set(v, object.ExceptionValue(t.exc))
	}
	t.outer = in.rescuing
	in.rescuing = t.exc
	m.pc = int(bg.bodies[i])
}

// rescueTest runs an fRescueTest: the classes of a rescue clause are on
// top as a list.
func (m *machine) rescueTest(ins *flatIns) error {
	bg := m.flat.scopes[ins.scope].begin
	t := &m.tries[bg.level]
	for _, cv := range m.pop().List().Elems {
		cls, ok := cv.AsClass()
		if !ok {
			return m.beginLeaves(bg, m.in.raise(object.TypeError, 0, "class or module required for rescue clause"))
		}
		if t.exc.Cls.IsA(cls) {
			m.rescued(bg, int(ins.b))
			return nil
		}
	}
	return m.rescueFlat(bg, int(ins.b)+1)
}

// atReal returns whether the next flat instruction of m is one a run
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
	// An activation inside the lists of an fTree is after that
	// instruction, and has the cursors of those lists already.
	inside := append([]cursor(nil), m.cursors...)
	called = called || len(inside) > 0
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
		if l.try >= 0 {
			t := &m.tries[l.try]
			cur.base = t.base
			switch l.phase {
			case phaseClasses:
				cur.exc = t.exc
			case phaseRescue:
				cur.exc, cur.outer = t.exc, t.outer
			case phaseEnsure:
				cur.pending = t.pending
			}
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
	m.cursors = append(m.cursors, inside...)
	m.flat, m.pc, m.bases, m.tries = nil, 0, nil, nil
}
