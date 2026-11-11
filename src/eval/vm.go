package eval

import (
	"math/big"
	"sync/atomic"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// The IR evaluator: it runs the instructions of a proc (cameo/ir.proto) on
// the value stack of one activation. doc/design/ir.md has the design.
//
// While the IR is being built, a method, a block or a top-level statement
// runs here when the compiler compiles all of it, and on the tree walker
// otherwise. Both evaluators run on the same scopes, frames and procs, so
// code on one calls code on the other.

// TreeWalkerOnly, when set, keeps every program on the tree walker. The
// tests set it to hold the two evaluators to the same output.
var TreeWalkerOnly atomic.Bool

// irProc is one proc of a unit: the code of a method, a block or a
// statement.
type irProc struct {
	unit *cameopb.Unit
	proc *cameopb.Proc
}

// irKey names what was compiled: a node, and whether as a top-level
// statement. A def at the top level is both a statement and, once it ran,
// a method's body.
type irKey struct {
	node      syntax.Node
	statement bool
}

// irFor returns the IR of the node of key, a method, a block or a
// top-level statement, which compile compiles on first use; nil when the
// compiler does not compile it yet.
func (c *code) irFor(key irKey, compile func() (*cameopb.Unit, error)) *irProc {
	if TreeWalkerOnly.Load() {
		return nil
	}
	if p, ok := c.ir.Load(key); ok {
		return p.(*irProc)
	}
	var p *irProc
	if unit, err := compile(); err == nil {
		p = &irProc{unit: unit, proc: unit.Procs[0]}
	}
	actual, _ := c.ir.LoadOrStore(key, p)
	return actual.(*irProc)
}

// IRCounts returns how many of the methods, blocks and top-level
// statements the program has run so far were compiled to the IR, and how
// many stayed on the tree walker.
func (in *Interp) IRCounts() (compiled, walked int) {
	in.code.ir.Range(func(_, p any) bool {
		if p.(*irProc) != nil {
			compiled++
		} else {
			walked++
		}
		return true
	})
	return compiled, walked
}

func (c *code) irDef(def *syntax.Def) *irProc {
	return c.irFor(irKey{node: def}, func() (*cameopb.Unit, error) { return ir.CompileDef("", def) })
}

func (c *code) irBlock(b *syntax.Block) *irProc {
	return c.irFor(irKey{node: b}, func() (*cameopb.Unit, error) { return ir.CompileBlock("", b) })
}

func (c *code) irStatement(n syntax.Node) *irProc {
	return c.irFor(irKey{node: n, statement: true}, func() (*cameopb.Unit, error) { return ir.CompileStatement("", n) })
}

// evalTop evaluates a top-level statement: its IR when it has one.
func (in *Interp) evalTop(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	if p := in.code.irStatement(n); p != nil {
		return in.runList(p.unit, p.proc.GetCode(), env, fr)
	}
	return in.eval(n, env, fr)
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
	case cameopb.Op_DROP:
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
		return &control{kind: ctrlReturn, value: m.pop(), target: fr, line: line}
	case cameopb.Op_BREAK:
		return &control{kind: ctrlBreak, value: m.pop(), line: line}
	case cameopb.Op_NEXT:
		return &control{kind: ctrlNext, value: m.pop(), line: line}
	case cameopb.Op_REDO:
		return &control{kind: ctrlRedo, line: line}
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
	case *cameopb.Value_BoolValue:
		return object.Bool(k.BoolValue), nil
	case *cameopb.Value_IntValue:
		return object.Int(k.IntValue), nil
	case *cameopb.Value_FloatValue:
		return object.Float(k.FloatValue), nil
	case *cameopb.Value_StringValue:
		return object.String(k.StringValue), nil
	case *cameopb.Value_BigValue:
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
			p := &irProc{unit: m.unit, proc: m.unit.Procs[k.Proc]}
			blockEnv := m.env
			if !p.proc.GetShared() {
				blockEnv = m.env.snapshot()
			}
			block = procValue(&Proc{IR: p, Env: blockEnv, Frame: fr})
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
