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
// call, a top-level statement.
type machine struct {
	in    *Interp
	unit  *cameopb.Unit
	env   *Env
	fr    *Frame
	stack []object.Value
}

// runBody runs the body of a proc in env and fr and returns its value.
// The debugger is told each statement's line, the first one's too.
func (in *Interp) runBody(p *irProc, env *Env, fr *Frame) (object.Value, error) {
	m := &machine{in: in, unit: p.unit, env: env, fr: fr}
	if err := m.body(p.proc.GetCode()); err != nil {
		return object.Value{}, err
	}
	return m.pop(), nil
}

// runList runs a list that leaves one value, in env and fr, and returns
// the value: a parameter's default, or a statement the caller told the
// debugger about.
func (in *Interp) runList(unit *cameopb.Unit, code []*cameopb.Code, env *Env, fr *Frame) (object.Value, error) {
	m := &machine{in: in, unit: unit, env: env, fr: fr}
	if err := m.list(code); err != nil {
		return object.Value{}, err
	}
	return m.pop(), nil
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

func has(c *cameopb.Code, f cameopb.Flag) bool { return c.Flags&uint64(f) != 0 }

// body runs a list of statements, telling the debugger the line of each
// before it runs.
func (m *machine) body(code []*cameopb.Code) error { return m.run(code, true) }

// list runs a list that is not a body: a condition, an argument. A
// statement inside it, after a DROP, is still the debugger's.
func (m *machine) list(code []*cameopb.Code) error { return m.run(code, false) }

func (m *machine) run(code []*cameopb.Code, statement bool) error {
	for _, c := range code {
		if statement && m.in.dbg != nil {
			if err := m.in.dbg.line(int(c.Line), m.env, m.fr); err != nil {
				return err
			}
		}
		statement = c.Op == cameopb.Op_DROP
		if err := m.exec(c); err != nil {
			return err
		}
	}
	return nil
}

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
		if v, ok := env.Get(m.name(c, 0)); ok {
			m.push(v)
		} else {
			m.push(object.Nil)
		}
	case cameopb.Op_STORE:
		name, v := m.name(c, 0), m.top()
		env.Set(name, v)
		if has(c, cameopb.Flag_FLAG_FORWARD) {
			// The lambda reads its own name, which was unbound when
			// it captured its scope: fill it in now.
			if p, ok := asProc(v); ok && p.Env != nil {
				p.Env.bind(name, v)
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
		m.push(object.MapValue(object.NewMap()))
	case cameopb.Op_MAP_SET:
		k := m.pop()
		v := m.pop()
		mp := m.pop().Map()
		m.push(object.MapValue(mp.Put(k, v)))
	case cameopb.Op_MAP_MERGE:
		v := m.pop()
		mp := m.pop().Map()
		if !v.IsNil() {
			other, ok := v.AsMap()
			if !ok {
				return in.raise(object.TypeError, line, "no implicit conversion of %s into Map", typeName(v))
			}
			for i, k := range other.Keys() {
				mp = mp.Put(k, other.Values()[i])
			}
		}
		m.push(object.MapValue(mp))
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
		v, err := in.interpolate(len(c.Code), line, func(i int) (object.Value, error) {
			if err := m.list(c.Code[i].GetCode()); err != nil {
				return object.Value{}, err
			}
			return m.pop(), nil
		})
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_LAMBDA:
		p := &irProc{unit: m.unit, proc: m.unit.Procs[c.Args[0].GetProc()]}
		m.push(procValue(&Proc{IR: p, Env: env.snapshot(), Frame: fr, Strict: has(c, cameopb.Flag_FLAG_STRICT)}))
	case cameopb.Op_SEND:
		return m.send(c)
	case cameopb.Op_SAFE:
		if !m.top().IsNil() {
			return m.list(c.Code[0].GetCode())
		}
	case cameopb.Op_BLOCK_GIVEN:
		m.push(object.Bool(!fr.Block.IsZero()))
	case cameopb.Op_NEED_BLOCK:
		if fr.Block.IsZero() {
			return in.raise(object.LocalJumpError, line, "no block given (yield)")
		}
	case cameopb.Op_YIELD:
		args, kw := m.callArgs(c, 0)
		v, err := in.callBlock(fr.Block, args, kw, object.Value{})
		if err != nil {
			return withLine(err, line)
		}
		m.push(v)
	case cameopb.Op_DROP, cameopb.Op_POP:
		m.pop()
	case cameopb.Op_IF:
		if m.pop().Truthy() {
			return m.body(c.Code[0].GetCode())
		}
		return m.body(c.Code[1].GetCode())
	case cameopb.Op_WHILE:
		return m.while(c)
	case cameopb.Op_AND:
		if m.top().Truthy() {
			m.pop()
			return m.list(c.Code[0].GetCode())
		}
	case cameopb.Op_OR:
		if !m.top().Truthy() {
			m.pop()
			return m.list(c.Code[0].GetCode())
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
		return m.begin(c)
	case cameopb.Op_CASE:
		return m.caseWhen(c)
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
		v, err := m.regexp(c)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_VAR_OR_CALL:
		name := m.name(c, 0)
		if v, ok := env.Get(name); ok {
			m.push(v)
			return nil
		}
		v, err := in.dispatch(fr, fr.Self, name, nil, nil, object.Value{}, true, true, true, line)
		if err != nil {
			return withLine(err, line)
		}
		m.push(v)
	case cameopb.Op_DEFINED_LOCAL:
		if _, ok := env.Get(m.name(c, 0)); ok {
			m.push(object.String("local-variable"))
		} else {
			m.push(object.Nil)
		}
	case cameopb.Op_DEFINED_CONST:
		base := len(m.stack)
		if err := m.list(c.Code[0].GetCode()); err != nil {
			if exc, ok := err.(*object.Exception); ok && exc.Cls.IsA(object.NameError) {
				m.stack = m.stack[:base]
				m.push(object.Nil)
				return nil
			}
			return err
		}
		m.pop()
		m.push(object.String("constant"))
	case cameopb.Op_DEFINED_METHOD:
		recv := fr.Self
		if recvCode := c.Code[0].GetCode(); len(recvCode) > 0 {
			base := len(m.stack)
			if err := m.list(recvCode); err != nil {
				if _, ok := err.(*object.Exception); ok {
					m.stack = m.stack[:base]
					m.push(object.Nil)
					return nil
				}
				return err
			}
			recv = m.pop()
		}
		if recv.Class().Lookup(m.name(c, 0)) != nil {
			m.push(object.String("method"))
		} else {
			m.push(object.Nil)
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
		args, kw := fr.Args, fr.Kw
		if !has(c, cameopb.Flag_FLAG_FORWARD) {
			args, kw = m.callArgs(c, 0)
		}
		if has(c, cameopb.Flag_FLAG_BLOCK) {
			block = m.closure(c.Args[1].GetProc())
		}
		v, err := in.callSuper(fr, args, kw, block, line)
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
		v, err := in.awaitOne(p)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_AWAIT:
		v, err := in.await(m.pop(), line)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_AWAIT_ALL:
		v, err := in.awaitAll(env)
		if err != nil {
			return err
		}
		m.push(v)
	case cameopb.Op_DEF:
		body := &irProc{unit: m.unit, proc: m.unit.Procs[c.Args[1].GetProc()]}
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
	p := &irProc{unit: m.unit, proc: m.unit.Procs[index]}
	env := m.env
	if !p.proc.GetShared() {
		env = env.snapshot()
	}
	return procValue(&Proc{IR: p, Env: env, Frame: m.fr})
}

// send runs a SEND: a method call.
func (m *machine) send(c *cameopb.Code) error {
	in, fr := m.in, m.fr
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
	if has(c, cameopb.Flag_FLAG_OPERATOR) {
		if v := binaryOp(name)(recv, args[0]); !v.IsZero() {
			m.push(v)
			return nil
		}
		v, err := in.call(fr, recv, name, args, nil, object.Value{}, false, line)
		if err != nil {
			return withLine(err, line)
		}
		m.push(v)
		return nil
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
	if !selfCall && mayMutate(name) {
		if v, handled, err := in.mutating(fr, recv, recvName, name, args, kw, block, has(c, cameopb.Flag_FLAG_REBINDS), line); handled {
			if err != nil {
				return err
			}
			m.push(v)
			return nil
		}
	}
	v, err := in.dispatch(fr, recv, name, args, kw, block, selfCall, has(c, cameopb.Flag_FLAG_VCALL), selfCall || has(c, cameopb.Flag_FLAG_PRIVATE), line)
	if err == nil && !selfCall && has(c, cameopb.Flag_FLAG_DROPPED) {
		err = in.dropped(recv, name, line)
	}
	if err != nil {
		return withLine(err, line)
	}
	m.push(v)
	return nil
}

// while runs a WHILE: while, until and their begin … end forms. A break,
// next or redo out of the middle of an expression leaves the stack as it
// was when the loop started.
func (m *machine) while(c *cameopb.Code) error {
	in := m.in
	cond, body := c.Code[0].GetCode(), c.Code[1].GetCode()
	base := len(m.stack)
	for first := true; ; first = false {
		if err := in.stopped(); err != nil {
			return err
		}
		if !has(c, cameopb.Flag_FLAG_DO_WHILE) || !first {
			if err := m.list(cond); err != nil {
				return err
			}
			if !m.pop().Truthy() {
				m.push(object.Nil)
				return nil
			}
		}
	pass:
		err := m.body(body)
		if err == nil {
			m.pop()
			continue
		}
		if ctl, ok := err.(*control); ok && ctl.target == nil {
			m.stack = m.stack[:base]
			switch ctl.kind {
			case ctrlBreak:
				m.push(ctl.value)
				return nil
			case ctrlNext:
				continue
			case ctrlRedo:
				if err := in.stopped(); err != nil {
					return err
				}
				goto pass
			}
		}
		return err
	}
}
