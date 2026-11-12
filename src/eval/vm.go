package eval

import (
	"errors"
	"math/big"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// The evaluator: it runs the instructions of a proc (cameo/ir.proto) on
// the value stack of one activation. A program is compiled to a unit
// before it runs (package ir). doc/design/ir.md has the design.

// irProc is one proc of a unit: the code of a method, a block, a class
// body or the top level.
type irProc struct {
	unit *cameopb.Unit
	proc *cameopb.Proc
	// index is the proc's place in the unit's procs.
	index uint32
}

// procOf returns proc index of unit.
func procOf(unit *cameopb.Unit, index uint32) *irProc {
	return &irProc{unit: unit, proc: unit.Procs[index], index: index}
}

// program is a parsed program compiled: its unit, and the code of each
// statement of its top level, which is proc 0 of the unit.
type program struct {
	unit       *cameopb.Unit
	statements [][]*cameopb.Code
}

// program returns prog compiled into a unit, once per parsed program.
func (in *Interp) program(file string, prog *syntax.Program) (*program, error) {
	if p, ok := in.code.programs.Load(prog); ok {
		return p.(*program), nil
	}
	unit, err := ir.Compile(file, prog)
	if err != nil {
		// The one node without an instruction is a splat outside a
		// list, which the checker refuses.
		var unsupported *ir.Unsupported
		if errors.As(err, &unsupported) {
			return nil, in.raise(object.CameoError, unsupported.Node.Line(), "a splat belongs in a list or an argument list")
		}
		return nil, err
	}
	p := &program{unit: unit}
	if len(prog.Body) > 0 {
		p.statements = statements(unit.Procs[0].GetCode())
	}
	in.code.units.LoadOrStore(unit.GetDigest(), unit)
	actual, _ := in.code.programs.LoadOrStore(prog, p)
	return actual.(*program), nil
}

// statements splits the code of a body into its statements, which a DROP
// ends.
func statements(code []*cameopb.Code) [][]*cameopb.Code {
	var out [][]*cameopb.Code
	start := 0
	for i, c := range code {
		if c.Op == cameopb.Op_DROP {
			out = append(out, code[start:i])
			start = i + 1
		}
	}
	return append(out, code[start:])
}

// runProgram runs the statements of prog with env and fr and returns the
// last value; a return among them ends them with its value.
func (in *Interp) runProgram(file string, prog *syntax.Program, env *Env, fr *Frame) (object.Value, error) {
	p, err := in.program(file, prog)
	if err != nil {
		return object.Value{}, err
	}
	var v object.Value = object.Nil
	for _, statement := range p.statements {
		if v, err = in.runList(p.unit, statement, env, fr); err != nil {
			if c, ok := err.(*control); ok && c.kind == ctrlReturn {
				return c.value, nil
			}
			return object.Value{}, err
		}
	}
	return v, nil
}

// machine runs the instructions of one activation: a method call, a block
// call, a top-level statement. cursors is where it is: the lists it is
// inside, outermost first (cursor.go).
type machine struct {
	in   *Interp
	unit *cameopb.Unit
	// syms is the names of unit as Syms, read when the first instruction
	// asks for one (sym).
	syms    *unitNames
	env     *Env
	fr      *Frame
	stack   []object.Value
	cursors []cursor
	// flat is the flat code the activation runs, nil when it runs from
	// cursors; pc is its next instruction, bases the height of the value
	// stack at the start of each loop it is in, and flatBody whether its
	// list is one of statements (flat.go). tries is what it keeps of each
	// BEGIN it is in. again is set when the block of the activation is to
	// run once more, for a redo.
	flat  *flatCode
	pc    int
	bases []int
	tries []try
	// own is the frame and the scope of a call of a method that keeps
	// neither (params.contained), and spare the slots of its scope
	// beyond the ones the scope holds itself.
	own      activation
	spare    []object.Value
	flatBody bool
	again    bool
	// kind says what the activation is, and proc is the block of an
	// actBlock; they decide what happens when it ends (stack.go).
	kind actKind
	proc *Proc
	// site is the instruction of the activation below that called this
	// one, nil when Go code did.
	site *cameopb.Code
	// debugged is set when the debugger was told of the activation.
	debugged bool
	// stepper is what an actStep is of.
	stepper *stepping
	// result is what the caller gets in place of the activation's own
	// value, when it is set: the instance new made, for an initialize.
	result object.Value
	// wait is what the activation awaits, while it does (await.go).
	wait *awaiting
	// passing is set when the activation made a tail call and was kept
	// (Interp.tailFrames): all it does yet is hand on the value. row is
	// then how many such activations there are in a row up to it.
	passing bool
	row     int
	// inGo counts the lists the activation runs to their end inside an
	// instruction (nested). A call made while it is not zero cannot go
	// back to the loop, and runs in a loop of its own.
	inGo int
}

// runBody runs the body of a proc in env and fr and returns its value.
// The debugger is told each statement's line, the first one's too.
func (in *Interp) runBody(p *irProc, env *Env, fr *Frame) (object.Value, error) {
	return in.runCode(p.unit, p.proc.GetCode(), env, fr, true)
}

// runList runs a list that leaves one value, in env and fr, and returns
// the value: a parameter's default, or a statement the caller told the
// debugger about.
func (in *Interp) runList(unit *cameopb.Unit, code []*cameopb.Code, env *Env, fr *Frame) (object.Value, error) {
	return in.runCode(unit, code, env, fr, false)
}

// runCode runs a list as an activation of its own; body is set for a list
// of statements.
func (in *Interp) runCode(unit *cameopb.Unit, code []*cameopb.Code, env *Env, fr *Frame, body bool) (object.Value, error) {
	m := in.machine()
	m.unit, m.env, m.fr = unit, env, fr
	// The list reads the variables of env by slot where env has the
	// layout of a proc, as the scope a default is evaluated in has, or
	// is the top level's.
	var flat *flatCode
	switch {
	case env == in.top:
		flat = in.flatOfLaid(unit, code, body, nil, env)
	case !env.own:
		flat = in.flatOfLaid(unit, code, body, env.layout, nil)
	default:
		flat = in.flatOf(unit, code, body)
	}
	m.startAt(flat, code, body)
	return in.start(m)
}

// startAt puts the activation at the start of code: of its flat code when
// it has some and no debugger follows the run, of a cursor otherwise.
func (m *machine) startAt(flat *flatCode, code []*cameopb.Code, body bool) {
	if flat != nil && m.in.dbg == nil {
		m.flat, m.pc, m.flatBody = flat, 0, body
		return
	}
	m.cursors = append(m.cursors, cursor{code: code, statement: body})
}

func (m *machine) push(v object.Value) { m.stack = append(m.stack, v) }

func (m *machine) pop() object.Value {
	v := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return v
}

func (m *machine) top() object.Value { return m.stack[len(m.stack)-1] }

// popN pops the n values on top of the stack, the first pushed first, into
// a slice of their own.
func (m *machine) popN(n int) []object.Value {
	if n == 0 {
		return nil
	}
	out := make([]object.Value, n)
	copy(out, m.stack[len(m.stack)-n:])
	m.stack = m.stack[:len(m.stack)-n]
	return out
}

// name is the name argument i of c.
func (m *machine) name(c *cameopb.Code, i int) string {
	return m.unit.Names[c.Args[i].GetName()]
}

// sym is the name argument i of c as a Sym.
func (m *machine) sym(c *cameopb.Code, i int) object.Sym {
	if m.syms == nil {
		m.syms = m.in.symsOf(m.unit)
	}
	return m.syms.syms[c.Args[i].GetName()]
}

// mayMutate reports whether the name a SEND calls can be one of Ruby's
// mutators (mutate.go).
func (m *machine) mayMutate(c *cameopb.Code) bool {
	if m.syms == nil {
		m.syms = m.in.symsOf(m.unit)
	}
	return m.syms.mutators[c.Args[0].GetName()]
}

func has(c *cameopb.Code, f cameopb.Flag) bool { return c.Flags&uint64(f) != 0 }

// exec runs one instruction.
func (m *machine) exec(c *cameopb.Code) error {
	in, env, fr := m.in, m.env, m.fr
	line := int(c.Line)
	switch c.Op {
	case cameopb.Op_CONST:
		v, err := m.constant(c)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_SYM:
		m.push(object.Symbol(m.name(c, 0)))
	case cameopb.Op_SELF:
		m.push(fr.Self)
	case cameopb.Op_LOAD:
		if v, ok := env.GetSym(m.sym(c, 0)); ok {
			m.push(v)
		} else {
			m.push(object.Nil)
		}
	case cameopb.Op_STORE:
		name, v := m.sym(c, 0), m.top()
		env.SetSym(name, v)
		if has(c, cameopb.Flag_FLAG_FORWARD) {
			// The lambda reads its own name, which was unbound when
			// it captured its scope: fill it in now.
			if p, ok := asProc(v); ok && p.Env != nil {
				p.Env.bindSym(name, v)
			}
		}
	case cameopb.Op_GETCONST:
		v, err := in.lookupConst(m.name(c, 0), fr, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_LIST:
		m.push(object.NewList(m.popN(int(c.Args[0].GetInt()))...))
	case cameopb.Op_APPEND:
		v := m.pop()
		l := m.pop().List()
		m.push(object.NewList(append(append([]object.Value{}, l.Elems...), v)...))
	case cameopb.Op_SPLAT:
		v := m.pop()
		l := m.pop().List()
		elems, err := in.splat(append([]object.Value{}, l.Elems...), v, line)
		if err != nil {
			return err
		}
		m.push(object.NewList(elems...))
	case cameopb.Op_MAP:
		// The argument is how many keys the literal names, absent in a
		// dump from before it was added.
		n := 0
		if len(c.Args) > 0 {
			n = int(c.Args[0].GetInt())
		}
		m.push(object.MapValue(object.NewMapSized(n)))
	case cameopb.Op_MAP_SET:
		// The map MAP made is on this stack alone until the literal is
		// done, so it is filled in place.
		k := m.pop()
		v := m.pop()
		m.top().Map().Add(k, v)
	case cameopb.Op_MAP_MERGE:
		v := m.pop()
		mp := m.top().Map()
		if !v.IsNil() {
			other, ok := v.AsMap()
			if !ok {
				return in.raise(object.TypeError, line, "no implicit conversion of %s into Map", typeName(v))
			}
			for i, k := range other.Keys() {
				mp.Add(k, other.Values()[i])
			}
		}
	case cameopb.Op_RANGE:
		r := &object.Range{Exclusive: has(c, cameopb.Flag_FLAG_EXCLUSIVE)}
		if has(c, cameopb.Flag_FLAG_HI) {
			r.Hi = m.pop()
		}
		if has(c, cameopb.Flag_FLAG_LO) {
			r.Lo = m.pop()
		}
		m.push(object.RangeValue(r))
	case cameopb.Op_INTERP:
		m.interp(c)
	case cameopb.Op_LAMBDA:
		p := writtenProc(m.unit, c.Args[0].GetProc(), m.captured(c.Args[0].GetProc()), fr)
		p.Strict = has(c, cameopb.Flag_FLAG_STRICT)
		m.push(procValue(p))
	case cameopb.Op_SEND:
		return m.send(c)
	case cameopb.Op_BINARY_PLUS, cameopb.Op_BINARY_MINUS, cameopb.Op_BINARY_TIMES, cameopb.Op_BINARY_DIVIDE,
		cameopb.Op_BINARY_MODULO, cameopb.Op_BINARY_LT, cameopb.Op_BINARY_LE, cameopb.Op_BINARY_GT,
		cameopb.Op_BINARY_GE, cameopb.Op_BINARY_EQ, cameopb.Op_BINARY_NE:
		return m.binary(c)
	case cameopb.Op_SAFE:
		if !m.top().IsNil() {
			m.enter(c, 0, phaseValue, len(m.stack), false)
		}
	case cameopb.Op_BLOCK_GIVEN:
		m.push(object.Bool(!fr.Block.IsZero()))
	case cameopb.Op_NEED_BLOCK:
		if fr.Block.IsZero() {
			return in.raise(object.LocalJumpError, line, "no block given (yield)")
		}
	case cameopb.Op_YIELD:
		args, kw := m.callArgs(c, 0)
		// A block a script wrote becomes the activation that runs.
		if p, ok := asProc(fr.Block); ok && p.written() && m.inGo == 0 {
			act, err := in.blockAct(p, args, kw, object.Value{})
			if err == nil {
				err = in.pushAct(act, c)
			}
			return withLine(err, line)
		}
		v, err := in.callBlock(fr.Block, args, kw, object.Value{})
		if err != nil {
			return withLine(err, line)
		}
		m.push(v)
	case cameopb.Op_DROP, cameopb.Op_POP:
		m.pop()
	case cameopb.Op_IF:
		branch := 1
		if m.pop().Truthy() {
			branch = 0
		}
		m.enter(c, branch, phaseValue, len(m.stack), true)
	case cameopb.Op_WHILE:
		return m.while(c)
	case cameopb.Op_AND:
		if m.top().Truthy() {
			m.pop()
			m.enter(c, 0, phaseValue, len(m.stack), false)
		}
	case cameopb.Op_OR:
		if !m.top().Truthy() {
			m.pop()
			m.enter(c, 0, phaseValue, len(m.stack), false)
		}
	case cameopb.Op_NOT:
		m.push(object.Bool(!m.pop().Truthy()))
	case cameopb.Op_RETURN:
		v := m.pop()
		if fr.ended {
			return in.raise(object.LocalJumpError, line, "unexpected return")
		}
		return &control{kind: ctrlReturn, value: v, target: fr, line: line}
	case cameopb.Op_BREAK:
		return &control{kind: ctrlBreak, value: m.pop(), line: line}
	case cameopb.Op_NEXT:
		return &control{kind: ctrlNext, value: m.pop(), line: line}
	case cameopb.Op_REDO:
		return &control{kind: ctrlRedo, line: line}
	case cameopb.Op_RETRY:
		return &control{kind: ctrlRetry, line: line}
	case cameopb.Op_BEGIN:
		m.begin(c)
	case cameopb.Op_CASE:
		m.caseWhen(c)
	case cameopb.Op_CASE_IN:
		return m.caseIn(c)
	case cameopb.Op_MATCH:
		subject := m.pop()
		ok, err := m.match(c.Args[0].GetPattern(), subject, line)
		if err != nil {
			return err
		}
		switch {
		case has(c, cameopb.Flag_FLAG_TEST):
			m.push(object.Bool(ok))
		case !ok:
			s, _ := in.Inspect(subject)
			return in.raise(object.NoMatchingPattern, line, "%s", s)
		default:
			m.push(object.Nil)
		}
	case cameopb.Op_MULTI_ASSIGN:
		values := m.pop().List().Elems
		var result object.Value
		if c.Args[0].GetInt() == 1 {
			result = values[0]
			if l, ok := values[0].AsList(); ok {
				values = l.Elems
			}
		} else {
			result = object.NewList(values...)
		}
		names := make([]string, len(c.Args)-2)
		for i := range names {
			names[i] = m.name(c, i+2)
		}
		assignNames(names, int(c.Args[1].GetInt()), values, env)
		m.push(result)
	case cameopb.Op_REGEXP:
		return m.regexp(c)
	case cameopb.Op_VAR_OR_CALL:
		if v, ok := env.GetSym(m.sym(c, 0)); ok {
			m.push(v)
			return nil
		}
		name := m.name(c, 0)
		return m.call(c, in.lookupSym(fr.Self, m.sym(c, 0), true), fr.Self, name, nil, nil, object.Value{}, true, true, line)
	case cameopb.Op_DEFINED_LOCAL:
		if _, ok := env.GetSym(m.sym(c, 0)); ok {
			m.push(object.String("local-variable"))
		} else {
			m.push(object.Nil)
		}
	case cameopb.Op_DEFINED_CONST:
		m.enter(c, 0, phaseDefined, len(m.stack), false)
	case cameopb.Op_DEFINED_METHOD:
		if len(c.Code[0].GetCode()) > 0 {
			m.enter(c, 0, phaseDefined, len(m.stack), false)
		} else {
			m.definedMethod(c, fr.Self)
		}
	case cameopb.Op_DEFINED_YIELD:
		if fr.Block.IsZero() {
			m.push(object.Nil)
		} else {
			m.push(object.String("yield"))
		}
	case cameopb.Op_SCOPED:
		var v object.Value
		var err error
		if has(c, cameopb.Flag_FLAG_TOP) {
			v, err = in.topConst(m.name(c, 0), line)
		} else {
			v, err = in.scopedConst(m.pop(), m.name(c, 0), line)
		}
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_NEED_METHOD:
		if fr.Method == nil {
			return in.raise(object.LocalJumpError, line, "super called outside of method")
		}
	case cameopb.Op_SUPER:
		block := fr.Block
		if has(c, cameopb.Flag_FLAG_BLOCK_ARG) {
			v, err := in.toProc(m.pop(), line)
			if err != nil {
				return err
			}
			block = v
		}
		// The arguments a method was called with may be on the stack of
		// its caller (send); the method super reaches may keep them.
		args, kw := append([]object.Value(nil), fr.Args...), fr.Kw
		if !has(c, cameopb.Flag_FLAG_FORWARD) {
			args, kw = m.callArgs(c, 0)
		}
		if has(c, cameopb.Flag_FLAG_BLOCK) {
			block = m.closure(c.Args[1].GetProc())
		}
		// A method a script defined becomes the activation that runs.
		meth := in.superMethod(fr)
		if m.inGo == 0 && in.runs(meth, true) {
			act, err := in.methodAct(meth, fr.Self, args, kw, block, line)
			if err == nil {
				err = in.pushAct(act, c)
			}
			return withLine(err, line)
		}
		v, err := in.callSuperTo(meth, fr, args, kw, block, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_WITH:
		value := m.pop()
		key := m.pop()
		target := m.pop()
		v, err := in.with(target, key, value, has(c, cameopb.Flag_FLAG_INDEX))
		if err != nil {
			return withLine(err, line)
		}
		m.push(v)
	case cameopb.Op_NEED_CAMEO:
		return in.needCameo(false, m.name(c, 0), fr, line)
	case cameopb.Op_ASYNC, cameopb.Op_AWAIT_CALL:
		args, kw := m.callArgs(c, 0)
		method, err := in.asyncMethod(m.pop(), line)
		if err != nil {
			return err
		}
		p, err := in.startCall(m.pop(), method, args, kw, env, line)
		if err != nil {
			return err
		}
		if c.Op == cameopb.Op_ASYNC {
			m.push(object.Int(p.id))
			return nil
		}
		return m.await(&awaiting{ps: []*Promise{p}, single: true})
	case cameopb.Op_AWAIT:
		// One promise, or each of a list of them.
		v := m.pop()
		if l, ok := v.AsList(); ok {
			w := &awaiting{ps: make([]*Promise, l.Len())}
			for i := range w.ps {
				var err error
				if w.ps[i], err = in.promise(l.Elems[i], line); err != nil {
					return err
				}
			}
			return m.await(w)
		}
		p, err := in.promise(v, line)
		if err != nil {
			return err
		}
		return m.await(&awaiting{ps: []*Promise{p}, single: true})
	case cameopb.Op_AWAIT_ALL:
		return m.await(&awaiting{ps: in.unawaited(env)})
	case cameopb.Op_DEF:
		body := procOf(m.unit, c.Args[1].GetProc())
		v, err := in.defineMethod(m.name(c, 0), has(c, cameopb.Flag_FLAG_SINGLETON), body, fr, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_CLASS, cameopb.Op_MODULE:
		return m.define(c)
	case cameopb.Op_ALIAS:
		v, err := in.alias(m.name(c, 0), m.name(c, 1), fr, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_FIELD:
		return m.field(c)
	case cameopb.Op_ATTR:
		def := object.Nil
		if has(c, cameopb.Flag_FLAG_DEFAULT) {
			def = m.pop()
		}
		names := make([]string, len(c.Args))
		for i := range names {
			names[i] = m.name(c, i)
		}
		v, err := in.declareAttrs(names, def, fr, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_RESERVED:
		v, err := in.reserve(reserved{Number: c.Args[0].GetInt(), Name: m.name(c, 1), line: line}, fr)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_INCLUDE:
		v, err := in.include(m.pop(), has(c, cameopb.Flag_FLAG_EXTEND), fr, line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_SETCONST:
		return in.defineConst(m.name(c, 0), m.top(), fr, line)
	case cameopb.Op_FIELD_SET:
		return in.writeField(m.name(c, 0), m.top(), fr, line)
	default:
		return in.raise(object.CameoError, line, "the evaluator does not run %s yet", c.Op)
	}
	return nil
}

// constant is the value of a CONST. An Integer beyond 64 bits is made
// once per program.
func (m *machine) constant(c *cameopb.Code) (object.Value, error) {
	switch k := c.Args[0].GetConstant().GetKind().(type) {
	case nil:
		return object.Nil, nil
	case *cameopb.Constant_BoolValue:
		return object.Bool(k.BoolValue), nil
	case *cameopb.Constant_IntValue:
		return object.Int(k.IntValue), nil
	case *cameopb.Constant_FloatValue:
		return object.Float(k.FloatValue), nil
	case *cameopb.Constant_StringValue:
		return object.String(k.StringValue), nil
	case *cameopb.Constant_BytesValue:
		// A string literal that is not valid UTF-8; see ir.proto.
		return object.String(string(k.BytesValue)), nil
	case *cameopb.Constant_BigValue:
		return m.in.code.literal(c, func() (object.Value, error) {
			b, _ := new(big.Int).SetString(k.BigValue, 10)
			return object.BigInt(b), nil
		})
	}
	return object.Value{}, m.in.raise(object.CameoError, int(c.Line), "a constant of kind %T", c.Args[0].GetConstant().GetKind())
}

// callArgs pops the arguments of a call, YIELD or SUPER whose count is
// argument i: the keywords, nil when it has none, then the positional
// ones.
func (m *machine) callArgs(c *cameopb.Code, i int) ([]object.Value, *object.Map) {
	var kw *object.Map
	if has(c, cameopb.Flag_FLAG_KW) {
		kw = m.pop().Map()
	}
	if has(c, cameopb.Flag_FLAG_ARGV) {
		return m.pop().List().Elems, kw
	}
	return m.popN(int(c.Args[i].GetInt())), kw
}

// closure makes the block of a call from proc index of the unit: over a
// copy of the values in sight, or for the body of a for loop over the
// scope itself.
func (m *machine) closure(index uint32) object.Value {
	env := m.env
	if !m.unit.Procs[index].GetShared() {
		env = m.captured(index)
	}
	return procValue(writtenProc(m.unit, index, env, m.fr))
}

// captured returns the scope a block or lambda written here, proc index
// of the unit, closes over: the variables it reads, with the values
// they have now. Under a debugger, which shows the variables in sight,
// it is every variable.
func (m *machine) captured(index uint32) *Env {
	if m.in.dbg != nil {
		return m.env.snapshot()
	}
	ps := m.in.procParams(m.unit, index)
	return m.env.capture(ps.layout, ps.reads)
}

// binaryNames is the name of each operator a BINARY_ instruction runs,
// by its distance from BINARY_PLUS; the Syms from SymAdd on are in the
// same order.
var binaryNames = [...]string{"+", "-", "*", "/", "%", "<", "<=", ">", ">=", "==", "!="}

// binary runs a BINARY_ instruction: an operator on the two values on top
// of the stack. Two integers or two floats are computed where they are;
// an operator a class defines is a call like any other.
func (m *machine) binary(c *cameopb.Code) error {
	i := c.Op - cameopb.Op_BINARY_PLUS
	op := object.SymAdd + object.Sym(i)
	n := len(m.stack)
	if v := binary(op, m.stack[n-2], m.stack[n-1]); !v.IsZero() {
		m.stack[n-2] = v
		m.stack = m.stack[:n-1]
		return nil
	}
	args := m.popN(1)
	recv := m.pop()
	return m.call(c, m.in.lookupSym(recv, op, false), recv, binaryNames[i], args, nil, object.Value{}, false, true, int(c.Line))
}

// send runs a SEND: a method call.
func (m *machine) send(c *cameopb.Code) error {
	in, fr := m.in, m.fr
	if c.Flags&^sendPlain == 0 && plainArgs(c) {
		if called, err := m.sendContained(c); called {
			return err
		}
	}
	name, line := m.name(c, 0), int(c.Line)
	var block object.Value
	if has(c, cameopb.Flag_FLAG_BLOCK_ARG) {
		v, err := in.toProc(m.pop(), line)
		if err != nil {
			return err
		}
		block = v
	}
	args, kw := m.callArgs(c, 1)
	selfCall := has(c, cameopb.Flag_FLAG_SELF)
	recv := fr.Self
	if !selfCall {
		recv = m.pop()
	}
	recvName := "x"
	for _, a := range c.Args[2:] {
		switch k := a.GetKind().(type) {
		case *cameopb.Arg_Proc:
			block = m.closure(k.Proc)
		case *cameopb.Arg_Name:
			recvName = m.unit.Names[k.Name]
		}
	}
	if !selfCall && m.mayMutate(c) {
		if v, handled, err := in.mutating(fr, recv, recvName, name, args, kw, block, has(c, cameopb.Flag_FLAG_REBINDS), line); handled {
			if err != nil {
				return err
			}
			m.push(v)
			return nil
		}
	}
	return m.call(c, in.lookupSym(recv, m.sym(c, 0), selfCall), recv, name, args, kw, block, has(c, cameopb.Flag_FLAG_VCALL), selfCall || has(c, cameopb.Flag_FLAG_PRIVATE), line)
}

// sendPlain is the flags of a SEND that sendContained may run: a call
// with positional arguments alone and no block.
const sendPlain = uint64(cameopb.Flag_FLAG_SELF | cameopb.Flag_FLAG_VCALL | cameopb.Flag_FLAG_PRIVATE | cameopb.Flag_FLAG_TAIL)

// sendContained runs a SEND with positional arguments alone when it
// calls a builtin that keeps no slice of its arguments
// (object.Method.Borrows), or a method a script defined whose calls keep
// neither their frame nor their scope (params.contained). The arguments are not copied off
// the stack: the call reads them where the caller pushed them, above
// the top of the caller's stack, which nothing writes to until the call
// ended. It reports false when the call is another kind, and has done
// nothing then.
func (m *machine) sendContained(c *cameopb.Code) (bool, error) {
	in := m.in
	if m.inGo != 0 || in.dbg != nil {
		return false, nil
	}
	selfCall := has(c, cameopb.Flag_FLAG_SELF)
	argc, n := int(c.Args[1].GetInt()), len(m.stack)
	recv, base := m.fr.Self, n-argc
	if !selfCall {
		if m.mayMutate(c) {
			return false, nil
		}
		base--
		recv = m.stack[base]
	}
	meth := in.lookupSym(recv, m.sym(c, 0), selfCall)
	private := selfCall || has(c, cameopb.Flag_FLAG_PRIVATE)
	if meth != nil && meth.Body == nil && (meth.Borrows || argc == 0 && meth.Owner != object.ClassClass && meth.Owner != object.ProcClass) && (meth.Arity < 0 || meth.Arity == argc) && (!meth.Private || private) {
		// A builtin that keeps no slice of its arguments, or has none,
		// reads them where they are, and is called without the general
		// call's checks: it is no method of a script and takes no
		// block. Class#new and the methods that call a Proc are left
		// to the general call, which runs a script's initialize and a
		// script's block as activations.
		v, err := meth.Native(in, recv, m.stack[n-argc:n:n], nil, object.Value{})
		if err != nil {
			return true, withLine(err, int(c.Line))
		}
		m.stack = append(m.stack[:base], v)
		return true, nil
	}
	if !in.runs(meth, private) || meth.Owner == object.EnumeratorClass {
		return false, nil
	}
	code := meth.Body.(*irProc)
	ps := in.paramsOf(code)
	if !ps.contained || !ps.plain || len(ps.Positional) != argc {
		return false, nil
	}
	if has(c, cameopb.Flag_FLAG_TAIL) && m.kind != actList {
		// A tail call: the call that makes it does not stay, and its
		// stack, where the arguments are, goes with it. A method whose
		// frame and scope are the machine's own runs the method it
		// calls in them; any other tail call copies its arguments.
		if in.tailFrames > 0 {
			return false, nil
		}
		flat := in.flatOfProc(code)
		if m.kind != actMethod || m.fr != &m.own.frame || m.flat == nil || len(m.cursors) > 0 || flat == nil {
			return false, nil
		}
		return true, withLine(m.tailLoop(meth, code, ps, flat, recv, argc), int(c.Line))
	}
	args := m.stack[n-argc : n : n]
	m.stack = m.stack[:base]
	line := int(c.Line)
	act, err := in.methodAct(meth, recv, args, nil, object.Value{}, line)
	if err == nil {
		err = in.pushAct(act, c)
	}
	return true, withLine(err, line)
}

// plainArgs reports whether the arguments of the SEND c after its name
// and its count are names alone: the variable the receiver was read
// from, which a mutator's error names, and no block.
func plainArgs(c *cameopb.Code) bool {
	for _, a := range c.Args[2:] {
		if _, ok := a.GetKind().(*cameopb.Arg_Name); !ok {
			return false
		}
	}
	return true
}

// tailLoop runs a tail call of the method meth, with the argc values on
// top of the stack, in the activation m itself: a method whose frame and
// scope are the machine's own. The arguments move to the bottom of the
// stack, where the frame reads them, the frame and the scope become the
// ones of the new call, and the activation goes on at the start of the
// method's flat code. A method that calls itself in tail position is a
// loop.
func (m *machine) tailLoop(meth *object.Method, code *irProc, ps *params, flat *flatCode, recv object.Value, argc int) error {
	in := m.in
	if err := in.stopped(); err != nil {
		return err
	}
	n := len(m.stack)
	copy(m.stack, m.stack[n-argc:n])
	clear(m.stack[argc:n])
	m.stack = m.stack[:argc]
	args := m.stack[:argc:argc]
	env := &m.own.env
	if s := env.slots; cap(s) > cap(m.spare) && cap(s) <= maxKeptStack {
		m.spare = s
	}
	clear(m.spare[:cap(m.spare)])
	*env = Env{}
	in.frame(&m.own.frame, meth, recv, args, nil, object.Value{}, code)
	m.lay(env, ps.layout)
	for i := range ps.Positional {
		env.slots[ps.Positional[i].slot] = args[i]
	}
	m.unit, m.syms = code.unit, nil
	m.flat, m.pc, m.bases, m.tries = flat, 0, m.bases[:0], m.tries[:0]
	return nil
}

// call calls the method meth, which lookup found for name on recv, for
// the instruction c. A method a script defined becomes the activation
// that runs, and its value is pushed when it ends; any other call is
// made here and its value pushed.
func (m *machine) call(c *cameopb.Code, meth *object.Method, recv object.Value, name string, args []object.Value, kw *object.Map, block object.Value, vcall, private bool, line int) error {
	in := m.in
	// each with a block, on an enumerator an iterating method made when
	// it was called without one, is that method called with the block
	// (stdlib's enumFor). Where the method runs as an activation, a
	// script's or a stepper, it is called here as the script would have
	// called it, and not from inside Enumerator#each.
	if m.inGo == 0 && meth != nil && meth.Owner == object.EnumeratorClass && meth.Name == "each" && !block.IsZero() && len(args) == 0 {
		if e, ok := recv.AsEnumerator(); ok && e.Call != nil && e.Make != "" && e.Make == e.Meth {
			made := in.lookup(e.Recv, e.Make, false)
			if in.runs(made, true) || made != nil && made.Step != nil && (made.Arity < 0 || len(e.Args) == made.Arity) {
				return m.call(c, made, e.Recv, e.Make, e.Args, nil, block, false, true, line)
			}
		}
	}
	if m.inGo == 0 && in.runs(meth, private) {
		act, err := in.methodAct(meth, recv, args, kw, block, line)
		if err == nil {
			err = in.pushCall(m, act, c)
		}
		return withLine(err, line)
	}
	// new on a class whose initialize a script defined makes the instance
	// and runs initialize as an activation, whose caller gets the
	// instance.
	if init := in.initializer(meth, recv); init != nil && m.inGo == 0 {
		inst := object.InstanceValue(allocate(recv.Cls()))
		act, err := in.methodAct(init, inst, args, kw, block, line)
		if err == nil {
			act.result = inst
			err = in.pushCall(m, act, c)
		}
		return withLine(err, line)
	}
	// A block or a lambda a script wrote, called as a value with call,
	// () or [], is the block's activation, as a yield to it is.
	if m.inGo == 0 && meth != nil && meth.Owner == object.ProcClass && meth.Body == nil && callsProc(meth.Name) {
		if p, ok := asProc(recv); ok && p.written() {
			act, err := in.blockAct(p, args, kw, block)
			if err == nil {
				err = in.pushCall(m, act, c)
			}
			return withLine(err, line)
		}
	}
	// A builtin that is a stepper, called with a block, is an activation
	// too: its steps and its block's calls take turns in the loop.
	if m.inGo == 0 && meth != nil && meth.Step != nil && !block.IsZero() && (!meth.Private || private) && (meth.Arity < 0 || len(args) == meth.Arity) {
		return withLine(in.pushAct(in.stepAct(meth, recv, args, kw, block), c), line)
	}
	v, err := in.dispatchTo(meth, recv, name, args, kw, block, vcall, private, line)
	// A call whose value is dropped did nothing when Ruby's changes the
	// receiver and cameo's returns a copy (dropped); such a method is a
	// builtin's, so the check is not an activation's to make.
	if err == nil && c.Op == cameopb.Op_SEND && !has(c, cameopb.Flag_FLAG_SELF) && has(c, cameopb.Flag_FLAG_DROPPED) {
		err = in.dropped(recv, name, line)
	}
	if err != nil {
		return withLine(err, line)
	}
	m.push(v)
	return nil
}
