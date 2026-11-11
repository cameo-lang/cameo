package ir

import (
	"fmt"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Unsupported is the error for a node the compiler does not compile yet.
// While the IR is being built, the evaluator runs the method, block or
// statement the node is in on the tree walker instead.
type Unsupported struct{ Node syntax.Node }

func (e *Unsupported) Error() string {
	return fmt.Sprintf("line %d: %T does not compile to the IR yet", e.Node.Line(), e.Node)
}

// Compile compiles a program into a unit: proc 0 is its top level.
func Compile(file string, prog *syntax.Program) (*cameopb.Unit, error) {
	c := newCompiler(file)
	_, err := c.proc("<main>", nil, prog.Body, prog.Line(), false)
	return c.unit, err
}

// CompileDef compiles a method into a unit of its own: proc 0 is its body.
func CompileDef(file string, def *syntax.Def) (*cameopb.Unit, error) {
	c := newCompiler(file)
	_, err := c.proc(def.Name, &def.Params, def.Body, def.Line(), false)
	return c.unit, err
}

// CompileBlock compiles a block into a unit of its own: proc 0 is its
// body.
func CompileBlock(file string, b *syntax.Block) (*cameopb.Unit, error) {
	c := newCompiler(file)
	_, err := c.block(b)
	return c.unit, err
}

// CompileStatement compiles one statement into a unit of its own: proc 0
// evaluates it and leaves its value.
func CompileStatement(file string, n syntax.Node) (*cameopb.Unit, error) {
	c := newCompiler(file)
	_, err := c.proc("<main>", nil, []syntax.Node{n}, n.Line(), false)
	return c.unit, err
}

// IsOperator reports whether a call of name with a receiver and one plain
// argument is compiled with the operator flag: the arithmetic and
// comparison operators, which the evaluator computes on two Integers or
// two Floats without a method call.
func IsOperator(name string) bool {
	switch name {
	case "+", "-", "*", "/", "%", "<", "<=", ">", ">=", "==", "!=":
		return true
	}
	return false
}

type compiler struct {
	unit  *cameopb.Unit
	names map[string]uint32
}

func newCompiler(file string) *compiler {
	return &compiler{unit: &cameopb.Unit{Version: Version, File: file}, names: map[string]uint32{}}
}

// name returns the index of s in the unit's names, adding it when new.
func (c *compiler) name(s string) uint32 {
	if i, ok := c.names[s]; ok {
		return i
	}
	i := uint32(len(c.unit.Names))
	c.unit.Names = append(c.unit.Names, s)
	c.names[s] = i
	return i
}

func (c *compiler) nameArg(s string) *cameopb.Arg {
	return &cameopb.Arg{Kind: &cameopb.Arg_Name{Name: c.name(s)}}
}

func intArg(i int) *cameopb.Arg { return &cameopb.Arg{Kind: &cameopb.Arg_Int{Int: int64(i)}} }

func procArg(i uint32) *cameopb.Arg { return &cameopb.Arg{Kind: &cameopb.Arg_Proc{Proc: i}} }

func constArg(v *cameopb.Value) *cameopb.Arg {
	return &cameopb.Arg{Kind: &cameopb.Arg_Constant{Constant: v}}
}

// proc compiles a body into a new proc of the unit and returns its index.
// The index is taken before the body is compiled, so a proc comes before
// the procs of the blocks written in it.
func (c *compiler) proc(name string, params *syntax.Params, body []syntax.Node, line int, shared bool) (uint32, error) {
	p := &cameopb.Proc{Name: name, Line: int32(line), Shared: shared}
	index := uint32(len(c.unit.Procs))
	c.unit.Procs = append(c.unit.Procs, p)
	if params != nil {
		ps, err := c.params(params)
		if err != nil {
			return 0, err
		}
		p.Params = ps
	}
	code, err := c.body(body, line)
	if err != nil {
		return 0, err
	}
	p.Code = code.Code
	return index, nil
}

// block compiles a literal block or a lambda's body into a proc.
func (c *compiler) block(b *syntax.Block) (uint32, error) {
	return c.proc("block", &b.Params, b.Body, b.Line(), b.Shared)
}

func (c *compiler) params(p *syntax.Params) (*cameopb.Params, error) {
	out := &cameopb.Params{}
	for _, param := range p.Positional {
		pp := &cameopb.Params_Positional{Name: c.name(param.Name), Splat: param.Splat}
		if param.Default != nil {
			def, err := c.value(param.Default)
			if err != nil {
				return nil, err
			}
			pp.Default = def
		}
		out.Positional = append(out.Positional, pp)
	}
	for _, k := range p.Keywords {
		kp := &cameopb.Params_Keyword{Name: c.name(k.Name), Required: k.Required}
		if k.Default != nil {
			def, err := c.value(k.Default)
			if err != nil {
				return nil, err
			}
			kp.Default = def
		}
		out.Keywords = append(out.Keywords, kp)
	}
	if p.KwRest != "" {
		i := c.name(p.KwRest)
		out.KwRest = &i
	}
	if p.BlockParam != "" {
		i := c.name(p.BlockParam)
		out.Block = &i
	}
	return out, nil
}

// emitter collects the instructions of one list.
type emitter struct {
	code []*cameopb.Code
}

func (e *emitter) emit(op cameopb.Op, line int, args ...*cameopb.Arg) *cameopb.Code {
	c := &cameopb.Code{Op: op, Line: int32(line), Args: args}
	e.code = append(e.code, c)
	return c
}

func (e *emitter) seq() *cameopb.Seq { return &cameopb.Seq{Code: e.code} }

// body compiles statements into a list that leaves the value of the last
// one, nil when there are none. A DROP ends each statement but the last.
func (c *compiler) body(nodes []syntax.Node, line int) (*cameopb.Seq, error) {
	e := &emitter{}
	if err := c.statements(e, nodes, line); err != nil {
		return nil, err
	}
	return e.seq(), nil
}

func (c *compiler) statements(e *emitter, nodes []syntax.Node, line int) error {
	if len(nodes) == 0 {
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{}))
		return nil
	}
	for i, n := range nodes {
		if i > 0 {
			e.emit(cameopb.Op_DROP, n.Line())
		}
		if err := c.expr(e, n); err != nil {
			return err
		}
	}
	return nil
}

// value compiles one expression into a list of its own, which leaves its
// value.
func (c *compiler) value(n syntax.Node) (*cameopb.Seq, error) {
	e := &emitter{}
	if err := c.expr(e, n); err != nil {
		return nil, err
	}
	return e.seq(), nil
}

// optional compiles an expression that may be absent, which is nil then.
func (c *compiler) optional(e *emitter, n syntax.Node, line int) error {
	if n == nil {
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{}))
		return nil
	}
	return c.expr(e, n)
}

func flag(f cameopb.Flag, on bool) uint64 {
	if on {
		return uint64(f)
	}
	return 0
}

// expr compiles one expression: instructions that leave its value on the
// stack.
func (c *compiler) expr(e *emitter, n syntax.Node) error {
	line := n.Line()
	switch n := n.(type) {
	case *syntax.Nil:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{}))
	case *syntax.Bool:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{Kind: &cameopb.Value_BoolValue{BoolValue: n.Value}}))
	case *syntax.Int:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{Kind: &cameopb.Value_IntValue{IntValue: n.Value}}))
	case *syntax.BigInt:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{Kind: &cameopb.Value_BigValue{BigValue: n.Digits}}))
	case *syntax.Float:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{Kind: &cameopb.Value_FloatValue{FloatValue: n.Value}}))
	case *syntax.Str:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Value{Kind: &cameopb.Value_StringValue{StringValue: n.Value}}))
	case *syntax.Sym:
		e.emit(cameopb.Op_SYM, line, c.nameArg(n.Name))
	case *syntax.Self:
		e.emit(cameopb.Op_SELF, line)
	case *syntax.Var:
		e.emit(cameopb.Op_LOAD, line, c.nameArg(n.Name))
	case *syntax.Assign:
		if err := c.expr(e, n.Value); err != nil {
			return err
		}
		e.emit(cameopb.Op_STORE, line, c.nameArg(n.Name)).Flags = flag(cameopb.Flag_FLAG_FORWARD, n.Forward)
	case *syntax.Const:
		e.emit(cameopb.Op_GETCONST, line, c.nameArg(n.Name))
	case *syntax.Interp:
		code := e.emit(cameopb.Op_INTERP, line)
		for _, part := range n.Parts {
			s, err := c.value(part)
			if err != nil {
				return err
			}
			code.Code = append(code.Code, s)
		}
	case *syntax.List:
		_, _, err := c.args(e, n.Elems, line, true)
		return err
	case *syntax.Map:
		return c.mapEntries(e, n.Entries, line)
	case *syntax.Range:
		var flags uint64
		if n.Lo != nil {
			if err := c.expr(e, n.Lo); err != nil {
				return err
			}
			flags |= uint64(cameopb.Flag_FLAG_LO)
		}
		if n.Hi != nil {
			if err := c.expr(e, n.Hi); err != nil {
				return err
			}
			flags |= uint64(cameopb.Flag_FLAG_HI)
		}
		e.emit(cameopb.Op_RANGE, line).Flags = flags | flag(cameopb.Flag_FLAG_EXCLUSIVE, n.Exclusive)
	case *syntax.Call:
		return c.call(e, n)
	case *syntax.Lambda:
		index, err := c.block(n.Block)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_LAMBDA, line, procArg(index)).Flags = flag(cameopb.Flag_FLAG_STRICT, n.Strict)
	case *syntax.Yield:
		e.emit(cameopb.Op_NEED_BLOCK, line)
		argc, flags, err := c.callArgs(e, n.Args, n.KwArgs, line)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_YIELD, line, intArg(argc)).Flags = flags
	case *syntax.Return:
		return c.jump(e, cameopb.Op_RETURN, n.Value, line)
	case *syntax.Break:
		return c.jump(e, cameopb.Op_BREAK, n.Value, line)
	case *syntax.Next:
		return c.jump(e, cameopb.Op_NEXT, n.Value, line)
	case *syntax.Redo:
		e.emit(cameopb.Op_REDO, line)
	case *syntax.If:
		if err := c.expr(e, n.Cond); err != nil {
			return err
		}
		then, err := c.body(n.Then, line)
		if err != nil {
			return err
		}
		els, err := c.body(n.Else, line)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_IF, line).Code = []*cameopb.Seq{then, els}
	case *syntax.While:
		cond, err := c.value(n.Cond)
		if err != nil {
			return err
		}
		body, err := c.body(n.Body, line)
		if err != nil {
			return err
		}
		code := e.emit(cameopb.Op_WHILE, line)
		code.Code = []*cameopb.Seq{cond, body}
		code.Flags = flag(cameopb.Flag_FLAG_DO_WHILE, n.DoWhile)
	case *syntax.And:
		return c.shortCircuit(e, cameopb.Op_AND, n.Left, n.Right, line)
	case *syntax.Or:
		return c.shortCircuit(e, cameopb.Op_OR, n.Left, n.Right, line)
	case *syntax.Not:
		if err := c.expr(e, n.Value); err != nil {
			return err
		}
		e.emit(cameopb.Op_NOT, line)
	case *syntax.Seq:
		return c.statements(e, n.Body, line)
	default:
		return &Unsupported{Node: n}
	}
	return nil
}

// jump compiles return, break and next: the value, nil when absent, then
// the instruction.
func (c *compiler) jump(e *emitter, op cameopb.Op, value syntax.Node, line int) error {
	if err := c.optional(e, value, line); err != nil {
		return err
	}
	e.emit(op, line)
	return nil
}

func (c *compiler) shortCircuit(e *emitter, op cameopb.Op, left, right syntax.Node, line int) error {
	if err := c.expr(e, left); err != nil {
		return err
	}
	r, err := c.value(right)
	if err != nil {
		return err
	}
	e.emit(op, line).Code = []*cameopb.Seq{r}
	return nil
}

// args compiles a list of expressions. Without a splat each leaves its
// value and count says how many; with one, or when asList is set, they
// are built into one list and argv is set.
func (c *compiler) args(e *emitter, nodes []syntax.Node, line int, asList bool) (count int, argv bool, err error) {
	splat := false
	for _, n := range nodes {
		if _, ok := n.(*syntax.Splat); ok {
			splat = true
		}
	}
	if !splat {
		for _, n := range nodes {
			if err := c.expr(e, n); err != nil {
				return 0, false, err
			}
		}
		if asList {
			e.emit(cameopb.Op_LIST, line, intArg(len(nodes)))
		}
		return len(nodes), asList, nil
	}
	e.emit(cameopb.Op_LIST, line, intArg(0))
	for _, n := range nodes {
		if s, ok := n.(*syntax.Splat); ok {
			if err := c.expr(e, s.Value); err != nil {
				return 0, false, err
			}
			e.emit(cameopb.Op_SPLAT, s.Line())
			continue
		}
		if err := c.expr(e, n); err != nil {
			return 0, false, err
		}
		e.emit(cameopb.Op_APPEND, n.Line())
	}
	return 0, true, nil
}

// mapEntries compiles the entries of a map literal, or of keyword
// arguments, into one map. An entry's value is evaluated before its key.
func (c *compiler) mapEntries(e *emitter, entries []syntax.MapEntry, line int) error {
	e.emit(cameopb.Op_MAP, line)
	for _, entry := range entries {
		if err := c.expr(e, entry.Value); err != nil {
			return err
		}
		if entry.Key == nil {
			e.emit(cameopb.Op_MAP_MERGE, line)
			continue
		}
		if err := c.expr(e, entry.Key); err != nil {
			return err
		}
		e.emit(cameopb.Op_MAP_SET, line)
	}
	return nil
}

// callArgs compiles positional and keyword arguments, and returns the
// count and the flags of the instruction that takes them.
func (c *compiler) callArgs(e *emitter, args []syntax.Node, kwargs []syntax.MapEntry, line int) (int, uint64, error) {
	count, argv, err := c.args(e, args, line, false)
	if err != nil {
		return 0, 0, err
	}
	flags := flag(cameopb.Flag_FLAG_ARGV, argv)
	if len(kwargs) > 0 {
		if err := c.mapEntries(e, kwargs, line); err != nil {
			return 0, 0, err
		}
		flags |= uint64(cameopb.Flag_FLAG_KW)
	}
	return count, flags, nil
}

// call compiles a method call.
func (c *compiler) call(e *emitter, n *syntax.Call) error {
	line := n.Line()
	selfCall := n.Recv == nil
	if selfCall && n.Name == "block_given?" && len(n.Args) == 0 {
		e.emit(cameopb.Op_BLOCK_GIVEN, line)
		return nil
	}
	if !selfCall {
		if err := c.expr(e, n.Recv); err != nil {
			return err
		}
	}
	if n.Safe {
		// The arguments and the call run only for a receiver that is
		// not nil.
		rest := &emitter{}
		if err := c.send(rest, n); err != nil {
			return err
		}
		e.emit(cameopb.Op_SAFE, line).Code = []*cameopb.Seq{rest.seq()}
		return nil
	}
	return c.send(e, n)
}

// send compiles the arguments of a call and its SEND; the receiver is on
// the stack already.
func (c *compiler) send(e *emitter, n *syntax.Call) error {
	line := n.Line()
	selfCall := n.Recv == nil
	if isOperatorCall(n) {
		if err := c.expr(e, n.Args[0]); err != nil {
			return err
		}
		e.emit(cameopb.Op_SEND, line, c.nameArg(n.Name), intArg(1)).Flags = uint64(cameopb.Flag_FLAG_OPERATOR)
		return nil
	}
	argc, flags, err := c.callArgs(e, n.Args, n.KwArgs, line)
	if err != nil {
		return err
	}
	args := []*cameopb.Arg{c.nameArg(n.Name), intArg(argc)}
	switch {
	case n.Block != nil:
		index, err := c.block(n.Block)
		if err != nil {
			return err
		}
		if n.Name == "new" {
			// Class.new identifies the class it builds by its block.
			c.unit.Procs[index].Digest = syntax.Digest(n.Block)
		}
		args = append(args, procArg(index))
		flags |= uint64(cameopb.Flag_FLAG_BLOCK)
	case n.BlockArg != nil:
		if err := c.expr(e, n.BlockArg); err != nil {
			return err
		}
		flags |= uint64(cameopb.Flag_FLAG_BLOCK_ARG)
	}
	if !selfCall {
		// The receiver's name is for the message of a mutating call,
		// which says x for a receiver that has none.
		if name := syntax.RecvName(n.Recv); name != "x" {
			args = append(args, c.nameArg(name))
		}
		if _, ok := n.Recv.(*syntax.Self); ok {
			flags |= uint64(cameopb.Flag_FLAG_PRIVATE)
		}
	}
	flags |= flag(cameopb.Flag_FLAG_SELF, selfCall) |
		flag(cameopb.Flag_FLAG_VCALL, n.VCall) |
		flag(cameopb.Flag_FLAG_DROPPED, n.Dropped) |
		flag(cameopb.Flag_FLAG_REBINDS, n.Rebinds)
	e.emit(cameopb.Op_SEND, line, args...).Flags = flags
	return nil
}

// isOperatorCall reports whether n is an operator with a receiver and one
// plain argument, nothing else. A safe call `&.` is an ordinary call.
func isOperatorCall(n *syntax.Call) bool {
	if !IsOperator(n.Name) || n.Recv == nil || len(n.Args) != 1 || n.Block != nil || n.BlockArg != nil || len(n.KwArgs) > 0 || n.Safe {
		return false
	}
	_, splat := n.Args[0].(*syntax.Splat)
	return !splat
}
