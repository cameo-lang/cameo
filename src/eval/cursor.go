package eval

import (
	"unsafe"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Where an activation is: a cursor per list it is inside. An instruction
// that holds lists does not run them; it enters one, and says with a phase
// what it does when the list ends. doc/design/ir.md, "The activation
// stack", has the design.

// cursor is a list of instructions an activation is inside, and the index
// of the next one.
type cursor struct {
	code []*cameopb.Code
	next int
	// owner is the instruction the list belongs to, nil for the list the
	// activation started with; list says which of the owner's lists it
	// is, and phase what the owner does when the list ends.
	owner *cameopb.Code
	list  int
	phase phase
	// base is the height of the value stack when the owner started,
	// which a completion that leaves the list early cuts the stack to.
	base int
	// statement is set when the next instruction starts a statement,
	// which the debugger is told the line of.
	statement bool
	// exc is, for a BEGIN, the exception its rescue clauses are tried
	// for or the one a clause is handling, and outer the exception the
	// clause around was handling. pending is what an ensure clause
	// interrupted, which goes on when the clause ends.
	exc, outer *object.Exception
	pending    error
}

// phase says what the owner of a list does when the list ends.
type phase int

const (
	// phaseValue: nothing; the list's value is the instruction's.
	phaseValue phase = iota
	// phaseCond and phaseBody are a WHILE's condition and body;
	// phaseBody is a BEGIN's body too.
	phaseCond
	phaseBody
	// phaseElse, phaseEnsure, phaseClasses and phaseRescue are a BEGIN's
	// else and ensure clauses, and the classes and the body of a rescue
	// clause.
	phaseElse
	phaseEnsure
	phaseClasses
	phaseRescue
	// phasePart is a part of an INTERP or a REGEXP, phaseStatic the
	// source of a REGEXP that is made once.
	phasePart
	phaseStatic
	// phaseConds is the conditions of a when of a CASE, phaseGuard the
	// guard of a clause of a CASE_IN.
	phaseConds
	phaseGuard
	// phaseDefined is the expression of a defined?.
	phaseDefined
)

// enter makes list i of c the list the activation runs next. body is set
// for a list of statements.
func (m *machine) enter(c *cameopb.Code, i int, ph phase, base int, body bool) *cursor {
	m.cursors = append(m.cursors, cursor{code: c.Code[i].GetCode(), owner: c, list: i, phase: ph, base: base, statement: body})
	return &m.cursors[len(m.cursors)-1]
}

// step runs the next instruction of the innermost list, or, when that
// list is through, continues the instruction that owns it. It returns the
// completion that leaves the list, which unwind finds the taker of.
func (m *machine) step() error {
	cur := &m.cursors[len(m.cursors)-1]
	if cur.next == len(cur.code) {
		ended := *cur
		m.cursors = m.cursors[:len(m.cursors)-1]
		return m.ended(&ended)
	}
	c := cur.code[cur.next]
	cur.next++
	if cur.statement && m.in.dbg != nil {
		if err := m.in.dbg.line(int(c.Line), m.env, m.fr); err != nil {
			return err
		}
	}
	cur.statement = c.Op == cameopb.Op_DROP
	return m.exec(c)
}

// nested runs a list to its end inside an instruction and pops its
// value: the expression in a pattern, which is matched in Go. A
// completion no cursor of the list takes is returned.
func (m *machine) nested(code []*cameopb.Code) (object.Value, error) {
	floor := len(m.cursors)
	m.cursors = append(m.cursors, cursor{code: code, base: len(m.stack)})
	m.inGo++
	defer func() { m.inGo-- }()
	for len(m.cursors) > floor {
		if err := m.step(); err != nil {
			if err = m.unwind(err, floor); err != nil {
				return object.Value{}, err
			}
		}
	}
	return m.pop(), nil
}

// ended continues the instruction that owns a list which ran to its end.
func (m *machine) ended(cur *cursor) error {
	c := cur.owner
	if c == nil || cur.phase == phaseValue {
		return nil
	}
	in := m.in
	switch c.Op {
	case cameopb.Op_WHILE:
		if cur.phase == phaseCond {
			if !m.pop().Truthy() {
				m.push(object.Nil)
				return nil
			}
			m.enter(c, 1, phaseBody, cur.base, true)
			return nil
		}
		m.pop()
		return m.whileCond(c, cur.base)
	case cameopb.Op_BEGIN:
		return m.beginEnded(cur)
	case cameopb.Op_INTERP:
		return m.interpPart(c, cur.list)
	case cameopb.Op_REGEXP:
		if cur.phase == phaseStatic {
			source := m.pop()
			v, err := in.newRegexp(source.Str(), int(c.Args[0].GetInt()), int(c.Line))
			if err != nil {
				return err
			}
			actual, _ := in.code.literals.LoadOrStore(c, v)
			m.push(actual.(object.Value))
			return nil
		}
		return m.regexpPart(c, cur.list)
	case cameopb.Op_CASE:
		return m.caseConds(c, cur.list)
	case cameopb.Op_CASE_IN:
		g := m.pop()
		i := cur.list / 2
		if g.Truthy() == (c.Args[2*i+1].GetInt() == 2) {
			return m.caseInFrom(c, i+1)
		}
		m.pop()
		m.enter(c, 2*i+1, phaseValue, len(m.stack), true)
		return nil
	case cameopb.Op_DEFINED_CONST:
		m.pop()
		m.push(object.String("constant"))
		return nil
	case cameopb.Op_DEFINED_METHOD:
		m.definedMethod(c, m.pop())
		return nil
	}
	return nil
}

// unwind finds the cursor above floor that takes the completion err, and
// returns nil once one did: the activation goes on where that cursor's
// instruction says. Every cursor above it is popped. It returns the
// completion, which a clause it went through may have changed, when none
// takes it.
func (m *machine) unwind(err error, floor int) error {
	for len(m.cursors) > floor {
		cur := &m.cursors[len(m.cursors)-1]
		c := cur.owner
		if c != nil {
			switch c.Op {
			case cameopb.Op_WHILE:
				// The body's break, next and redo are the loop's; the
				// condition's are not.
				if ctl, ok := err.(*control); ok && ctl.target == nil && cur.phase == phaseBody {
					base := cur.base
					switch ctl.kind {
					case ctrlBreak:
						m.cursors = m.cursors[:len(m.cursors)-1]
						m.stack = m.stack[:base]
						m.push(ctl.value)
						return nil
					case ctrlNext:
						m.cursors = m.cursors[:len(m.cursors)-1]
						m.stack = m.stack[:base]
						if err = m.whileCond(c, base); err == nil {
							return nil
						}
						continue
					case ctrlRedo:
						m.cursors = m.cursors[:len(m.cursors)-1]
						m.stack = m.stack[:base]
						if err = m.in.stopped(); err == nil {
							m.enter(c, 1, phaseBody, base, true)
							return nil
						}
						continue
					}
				}
			case cameopb.Op_BEGIN:
				ended := *cur
				m.cursors = m.cursors[:len(m.cursors)-1]
				if err = m.beginLeft(&ended, err); err == nil {
					return nil
				}
				continue
			case cameopb.Op_DEFINED_CONST, cameopb.Op_DEFINED_METHOD:
				// defined? of an expression that raises is nil: a
				// constant that is not there, a receiver that fails.
				if exc, ok := err.(*object.Exception); ok && (c.Op == cameopb.Op_DEFINED_METHOD || exc.Cls.IsA(object.NameError)) {
					m.stack = m.stack[:cur.base]
					m.cursors = m.cursors[:len(m.cursors)-1]
					m.push(object.Nil)
					return nil
				}
			}
		}
		m.cursors = m.cursors[:len(m.cursors)-1]
	}
	return err
}

// whileCond starts the next pass of a WHILE: its condition.
func (m *machine) whileCond(c *cameopb.Code, base int) error {
	if err := m.in.stopped(); err != nil {
		return err
	}
	m.enter(c, 0, phaseCond, base, false)
	return nil
}

// while runs a WHILE: while, until and their begin … end forms. A break,
// next or redo out of the middle of an expression leaves the stack as it
// was when the loop started.
func (m *machine) while(c *cameopb.Code) error {
	base := len(m.stack)
	if has(c, cameopb.Flag_FLAG_DO_WHILE) {
		if err := m.in.stopped(); err != nil {
			return err
		}
		m.enter(c, 1, phaseBody, base, true)
		return nil
	}
	return m.whileCond(c, base)
}

// begin runs a BEGIN: begin/rescue/else/ensure. Its lists are the body,
// the else and the ensure, then the classes and the body of each rescue
// clause.
func (m *machine) begin(c *cameopb.Code) {
	m.enter(c, 0, phaseBody, len(m.stack), true)
}

// beginEnded continues a BEGIN one of whose lists ran to its end.
func (m *machine) beginEnded(cur *cursor) error {
	c := cur.owner
	switch cur.phase {
	case phaseBody:
		if has(c, cameopb.Flag_FLAG_ELSE) {
			m.pop()
			m.enter(c, 1, phaseElse, cur.base, true)
			return nil
		}
	case phaseClasses:
		// The classes of a rescue clause, evaluated when an exception
		// reached it.
		i := (cur.list - 3) / 2
		for _, cv := range m.pop().List().Elems {
			cls, ok := cv.AsClass()
			if !ok {
				return m.beginLeave(cur, m.in.raise(object.TypeError, 0, "class or module required for rescue clause"))
			}
			if cur.exc.Cls.IsA(cls) {
				m.rescue(cur, i)
				return nil
			}
		}
		return m.rescueFrom(cur, i+1)
	case phaseRescue:
		m.in.rescuing = cur.outer
	case phaseEnsure:
		m.pop()
		return cur.pending
	}
	if has(c, cameopb.Flag_FLAG_ENSURE) {
		m.enter(c, 2, phaseEnsure, cur.base, true)
	}
	return nil
}

// beginLeft continues a BEGIN one of whose lists a completion left: a
// rescue clause may take an exception out of the body, the body runs
// again for a retry out of a clause, and the ensure clause runs before
// the completion goes on. It returns nil when the BEGIN took the
// completion or its ensure clause runs first.
func (m *machine) beginLeft(cur *cursor, err error) error {
	c := cur.owner
	switch cur.phase {
	case phaseBody:
		if exc, ok := err.(*object.Exception); ok {
			m.stack = m.stack[:cur.base]
			cur.exc = exc
			return m.rescueFrom(cur, 0)
		}
	case phaseRescue:
		m.in.rescuing = cur.outer
		if ctl, ok := err.(*control); ok && ctl.kind == ctrlRetry {
			m.stack = m.stack[:cur.base]
			m.enter(c, 0, phaseBody, cur.base, true)
			return nil
		}
		err = caused(err, cur.exc)
	case phaseEnsure:
		// The ensure clause did not end: what it interrupted is gone.
		return err
	}
	return m.beginLeave(cur, err)
}

// beginLeave leaves a BEGIN with a completion none of its clauses took:
// through its ensure clause, when it has one.
func (m *machine) beginLeave(cur *cursor, err error) error {
	c := cur.owner
	if !has(c, cameopb.Flag_FLAG_ENSURE) {
		return err
	}
	m.stack = m.stack[:cur.base]
	m.enter(c, 2, phaseEnsure, cur.base, true).pending = err
	return nil
}

// rescueFrom tries the rescue clauses of a BEGIN from clause i on for the
// exception cur.exc: it enters the body of the first that fits, or the
// classes of the first that names some, which decide when they are
// evaluated. With no clause left the exception leaves the BEGIN.
func (m *machine) rescueFrom(cur *cursor, i int) error {
	c := cur.owner
	for ; 3+2*i < len(c.Code); i++ {
		if len(c.Code[3+2*i].GetCode()) > 0 {
			m.enter(c, 3+2*i, phaseClasses, cur.base, false).exc = cur.exc
			return nil
		}
		if cur.exc.Cls.IsA(object.StandardError) {
			m.rescue(cur, i)
			return nil
		}
	}
	return m.beginLeave(cur, cur.exc)
}

// rescue enters the body of rescue clause i of a BEGIN for cur.exc.
func (m *machine) rescue(cur *cursor, i int) {
	in, c, exc := m.in, cur.owner, cur.exc
	if in.rescuing != nil {
		// Raised inside another rescue clause and rescued before it
		// ends: it takes its cause where it is caught.
		exc = caused(exc, in.rescuing).(*object.Exception)
	}
	if v := m.name(c, i); v != "" {
		m.env.Set(v, object.ExceptionValue(exc))
	}
	body := m.enter(c, 4+2*i, phaseRescue, cur.base, true)
	body.exc, body.outer = exc, in.rescuing
	in.rescuing = exc
}

// interp runs an INTERP: a string with interpolation. The string so far
// is on the stack under each part while the part is evaluated, and a part
// is made a string before the next one is evaluated.
func (m *machine) interp(c *cameopb.Code) {
	m.push(object.String(""))
	if len(c.Code) > 0 {
		m.enter(c, 0, phasePart, len(m.stack), false)
	}
}

// interpPart adds part i of an INTERP, whose value is on the stack, to
// the string under it, and enters the next part.
func (m *machine) interpPart(c *cameopb.Code, i int) error {
	if err := m.interpAdd(c, i+1 == len(c.Code)); err != nil {
		return err
	}
	if i+1 < len(c.Code) {
		m.enter(c, i+1, phasePart, len(m.stack), false)
	}
	return nil
}

// interpAdd adds the value on top of the stack to the string under it,
// as a part of the INTERP c.
//
// The string so far is the machine's own until the INTERP is through:
// no script has it. It is kept in room that the next part is written
// into, so that an INTERP of n parts copies each part once and not the
// string so far n times. last says that v is the INTERP's last part:
// the string then keeps the room, and the machine lets go of it.
func (m *machine) interpAdd(c *cameopb.Code, last bool) error {
	in, line := m.in, int(c.Line)
	v := m.pop()
	sofar := m.pop()
	s, err := in.ToS(v)
	if err != nil {
		return withLine(err, line)
	}
	// The result takes the encoding its parts are compatible in, as
	// Ruby's does: a binary part in UTF-8 text makes it binary. UTF-8
	// parts alone, the usual case, need no looking at.
	kind := sofar.Kind()
	if pk := v.Kind(); v.IsString() && (pk != kind || kind != object.KindString) {
		next, ok := stdlib.CombineEncodings(kind, sofar.Str() == "", object.ASCIIOnly(sofar.Str()), pk, s == "", object.ASCIIOnly(s))
		if !ok {
			return in.raise(object.CompatibilityError, line, "%s", stdlib.IncompatibleEncodings(kind, pk))
		}
		kind = next
	}
	prev := sofar.Str()
	switch {
	case len(prev) == 0 && last:
		// One part: the string is the part.
		m.push(object.StringOf(kind, s))
		return nil
	case len(prev) > 0 && len(prev) == len(m.sofar) && unsafe.StringData(prev) == unsafe.SliceData(m.sofar):
		// The string so far is the one the part before left in the
		// room: this part goes behind it.
		m.sofar = append(m.sofar, s...)
	default:
		// The first part, or a string so far that is not in the room:
		// the INTERP of a part came between, or a resume.
		m.sofar = append(append(make([]byte, 0, max(2*(len(prev)+len(s)), 32)), prev...), s...)
	}
	m.push(object.StringOf(kind, unsafe.String(unsafe.SliceData(m.sofar), len(m.sofar))))
	if last {
		m.sofar = nil
	}
	return nil
}

// regexp runs a REGEXP: a regexp literal. One without interpolation is
// made once per program; one with has its source so far on the stack
// under each part.
func (m *machine) regexp(c *cameopb.Code) error {
	if has(c, cameopb.Flag_FLAG_STATIC) {
		if v, ok := m.in.code.literals.Load(c); ok {
			m.push(v.(object.Value))
			return nil
		}
		m.enter(c, 0, phaseStatic, len(m.stack), false)
		return nil
	}
	m.push(object.String(""))
	return m.regexpNext(c, 0)
}

// regexpPart adds part i of a REGEXP, whose value is on the stack, to the
// source under it: a regexp embeds as its to_s, (?flags:source), as in
// Ruby, and any other value as its to_s.
func (m *machine) regexpPart(c *cameopb.Code, i int) error {
	v := m.pop()
	sofar := m.pop()
	var s string
	if re, ok := v.AsRegexp(); ok {
		s = re.String()
	} else {
		var err error
		if s, err = m.in.ToS(v); err != nil {
			return err
		}
	}
	m.push(object.String(sofar.Str() + s))
	return m.regexpNext(c, i+1)
}

// regexpNext enters part i of a REGEXP, or makes the regexp of the
// source on the stack when the parts are through.
func (m *machine) regexpNext(c *cameopb.Code, i int) error {
	if i < len(c.Code) {
		m.enter(c, i, phasePart, len(m.stack), false)
		return nil
	}
	v, err := m.in.newRegexp(m.pop().Str(), int(c.Args[0].GetInt()), int(c.Line))
	if err != nil {
		return err
	}
	m.push(v)
	return nil
}

// caseWhen runs a CASE: two lists per when, its conditions and its body,
// then the else. The subject stays on the stack until a body is entered.
func (m *machine) caseWhen(c *cameopb.Code) {
	m.caseFrom(c, 0)
}

// caseFrom enters the conditions of the when whose lists start at i, or
// the else when there is none left.
func (m *machine) caseFrom(c *cameopb.Code, i int) {
	last := len(c.Code) - 1
	if i < last {
		m.enter(c, i, phaseConds, len(m.stack), false)
		return
	}
	if has(c, cameopb.Flag_FLAG_SUBJECT) {
		m.pop()
	}
	m.enter(c, last, phaseValue, len(m.stack), true)
}

// caseConds tests the conditions of a when, list i of a CASE, which are
// on the stack as a list, and enters its body or goes to the next when.
func (m *machine) caseConds(c *cameopb.Code, i int) error {
	hit, err := m.caseHit(c)
	if err != nil {
		return err
	}
	if hit {
		m.enter(c, i+1, phaseValue, len(m.stack), true)
		return nil
	}
	m.caseFrom(c, i+2)
	return nil
}

// caseHit tests the conditions of a when of the CASE c, which are on the
// stack as a list, and drops them. With one that fits it drops the
// subject too and reports true.
func (m *machine) caseHit(c *cameopb.Code) (bool, error) {
	conds := m.pop()
	var subject object.Value
	if has(c, cameopb.Flag_FLAG_SUBJECT) {
		subject = m.top()
	}
	for _, cond := range conds.List().Elems {
		hit := cond.Truthy()
		if !subject.IsZero() {
			var err error
			if hit, err = m.in.caseEqual(cond, subject); err != nil {
				return false, withLine(err, int(c.Line))
			}
		}
		if hit {
			if !subject.IsZero() {
				m.pop()
			}
			return true, nil
		}
	}
	return false, nil
}

// caseIn runs a CASE_IN: per clause a pattern, the kind of its guard and
// two lists, the guard and the body; then the else, when it has one. The
// subject stays on the stack until a body is entered.
func (m *machine) caseIn(c *cameopb.Code) error {
	return m.caseInFrom(c, 0)
}

// caseInFrom tries the clauses of a CASE_IN from clause i on.
func (m *machine) caseInFrom(c *cameopb.Code, i int) error {
	in, line := m.in, int(c.Line)
	subject := m.top()
	for ; 2*i < len(c.Args); i++ {
		ok, err := m.match(c.Args[2*i].GetPattern(), subject, line)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if c.Args[2*i+1].GetInt() != 0 {
			m.enter(c, 2*i, phaseGuard, len(m.stack), false)
			return nil
		}
		m.pop()
		m.enter(c, 2*i+1, phaseValue, len(m.stack), true)
		return nil
	}
	m.pop()
	if has(c, cameopb.Flag_FLAG_ELSE) {
		m.enter(c, len(c.Code)-1, phaseValue, len(m.stack), true)
		return nil
	}
	s, _ := in.Inspect(subject)
	return in.raise(object.NoMatchingPattern, line, "%s", s)
}

// definedMethod pushes what defined? says of the method a DEFINED_METHOD
// names on recv.
func (m *machine) definedMethod(c *cameopb.Code, recv object.Value) {
	if recv.Class().Lookup(m.name(c, 0)) != nil {
		m.push(object.String("method"))
	} else {
		m.push(object.Nil)
	}
}
