package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Unsupported is the error for a node the IR has no instruction for: a
// splat outside a list or an argument list, which the checker refuses.
type Unsupported struct{ Node syntax.Node }

func (e *Unsupported) Error() string {
	return fmt.Sprintf("line %d: %T does not compile to the IR", e.Node.Line(), e.Node)
}

// Compile compiles a program into a unit: proc 0 is its top level.
func Compile(file string, prog *syntax.Program) (*cameopb.Unit, error) {
	c := newCompiler(file)
	if _, err := c.proc("<main>", nil, prog.Body, prog.Line(), false); err != nil {
		return nil, err
	}
	digest, err := Digest(c.unit)
	if err != nil {
		return nil, err
	}
	c.unit.Digest = digest
	return c.unit, nil
}

// Digest returns what identifies a unit: a hash of its wire form, with its
// digest field left out. Two units with one digest are the same code from
// the same file, line for line.
func Digest(u *cameopb.Unit) (string, error) {
	plain := proto.Clone(u).(*cameopb.Unit)
	plain.Digest = ""
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(plain)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8]), nil
}

// binaryOps is the instruction of each arithmetic and comparison
// operator, which the evaluator computes on two Integers or two Floats
// without a method call.
var binaryOps = map[string]cameopb.Op{
	"+":  cameopb.Op_BINARY_PLUS,
	"-":  cameopb.Op_BINARY_MINUS,
	"*":  cameopb.Op_BINARY_TIMES,
	"/":  cameopb.Op_BINARY_DIVIDE,
	"%":  cameopb.Op_BINARY_MODULO,
	"<":  cameopb.Op_BINARY_LT,
	"<=": cameopb.Op_BINARY_LE,
	">":  cameopb.Op_BINARY_GT,
	">=": cameopb.Op_BINARY_GE,
	"==": cameopb.Op_BINARY_EQ,
	"!=": cameopb.Op_BINARY_NE,
}

// IsOperator reports whether a call of name with a receiver and one plain
// argument is compiled to a BINARY_ instruction.
func IsOperator(name string) bool {
	_, ok := binaryOps[name]
	return ok
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

func constArg(v *cameopb.Constant) *cameopb.Arg {
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
	if err := c.sequence(e, nodes, line, cameopb.Op_DROP); err != nil {
		return nil, err
	}
	return e.seq(), nil
}

// sequence compiles expressions that run one after the other and leave
// the value of the last one, nil when there are none. between discards
// the value of each but the last: DROP for the statements of a body, POP
// for the expressions one construct was rewritten to.
func (c *compiler) sequence(e *emitter, nodes []syntax.Node, line int, between cameopb.Op) error {
	if len(nodes) == 0 {
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{}))
		return nil
	}
	for i, n := range nodes {
		if i > 0 {
			e.emit(between, n.Line())
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
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{}))
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
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{}))
	case *syntax.Bool:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_BoolValue{BoolValue: n.Value}}))
	case *syntax.Int:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_IntValue{IntValue: n.Value}}))
	case *syntax.BigInt:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_BigValue{BigValue: n.Digits}}))
	case *syntax.Float:
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_FloatValue{FloatValue: n.Value}}))
	case *syntax.Str:
		if !utf8.ValidString(n.Value) {
			// A protobuf string is UTF-8; a literal with other bytes in
			// it, "\xff", travels as bytes and is a String all the same.
			e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_BytesValue{BytesValue: []byte(n.Value)}}))
			break
		}
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_StringValue{StringValue: n.Value}}))
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
		return c.sequence(e, n.Body, line, cameopb.Op_POP)
	case *syntax.Begin:
		return c.begin(e, n)
	case *syntax.Retry:
		e.emit(cameopb.Op_RETRY, line)
	case *syntax.Case:
		return c.caseWhen(e, n)
	case *syntax.CaseIn:
		return c.caseIn(e, n)
	case *syntax.MatchPattern:
		if err := c.expr(e, n.Subject); err != nil {
			return err
		}
		p, err := c.pattern(n.Pattern)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_MATCH, line, &cameopb.Arg{Kind: &cameopb.Arg_Pattern{Pattern: p}}).Flags = flag(cameopb.Flag_FLAG_TEST, n.Test)
	case *syntax.MultiAssign:
		if _, _, err := c.args(e, n.Values, line, true); err != nil {
			return err
		}
		args := []*cameopb.Arg{intArg(len(n.Values)), intArg(n.SplatIndex)}
		for _, name := range n.Names {
			args = append(args, c.nameArg(name))
		}
		e.emit(cameopb.Op_MULTI_ASSIGN, line, args...)
	case *syntax.Regexp:
		code := e.emit(cameopb.Op_REGEXP, line, intArg(n.Options))
		for _, part := range n.Parts {
			s, err := c.value(part)
			if err != nil {
				return err
			}
			code.Code = append(code.Code, s)
		}
		if len(n.Parts) == 1 {
			_, static := n.Parts[0].(*syntax.Str)
			code.Flags = flag(cameopb.Flag_FLAG_STATIC, static)
		}
	case *syntax.VarOrCall:
		e.emit(cameopb.Op_VAR_OR_CALL, line, c.nameArg(n.Name))
	case *syntax.Defined:
		return c.defined(e, n)
	case *syntax.ScopedConst:
		if n.Scope == nil {
			e.emit(cameopb.Op_SCOPED, line, c.nameArg(n.Name)).Flags = uint64(cameopb.Flag_FLAG_TOP)
			return nil
		}
		if err := c.expr(e, n.Scope); err != nil {
			return err
		}
		e.emit(cameopb.Op_SCOPED, line, c.nameArg(n.Name))
	case *syntax.ConstAssign:
		if err := c.expr(e, n.Value); err != nil {
			return err
		}
		e.emit(cameopb.Op_SETCONST, line, c.nameArg(n.Name))
	case *syntax.FieldAssign:
		if err := c.expr(e, n.Value); err != nil {
			return err
		}
		e.emit(cameopb.Op_FIELD_SET, line, c.nameArg(n.Name))
	case *syntax.Super:
		return c.super(e, n)
	case *syntax.With:
		for _, operand := range []syntax.Node{n.Target, n.Key, n.Value} {
			if err := c.expr(e, operand); err != nil {
				return err
			}
		}
		e.emit(cameopb.Op_WITH, line).Flags = flag(cameopb.Flag_FLAG_INDEX, n.Index)
	case *syntax.Async:
		return c.asyncCall(e, cameopb.Op_ASYNC, "async", n.Qualified, n.Recv, n.Method, n.Args, n.KwArgs, line)
	case *syntax.Await:
		if n.Promise == nil {
			return c.asyncCall(e, cameopb.Op_AWAIT_CALL, "await", n.Qualified, n.Recv, n.Method, n.Args, n.KwArgs, line)
		}
		c.needCameo(e, "await", n.Qualified, line)
		if err := c.expr(e, n.Promise); err != nil {
			return err
		}
		e.emit(cameopb.Op_AWAIT, line).Flags = flag(cameopb.Flag_FLAG_QUALIFIED, n.Qualified)
	case *syntax.AwaitAll:
		c.needCameo(e, "await", n.Qualified, line)
		e.emit(cameopb.Op_AWAIT_ALL, line).Flags = flag(cameopb.Flag_FLAG_QUALIFIED, n.Qualified)
	case *syntax.Def:
		index, err := c.proc(n.Name, &n.Params, n.Body, line, false)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_DEF, line, c.nameArg(n.Name), procArg(index)).Flags = flag(cameopb.Flag_FLAG_SINGLETON, n.Singleton)
	case *syntax.ClassDef:
		flags, err := c.scope(e, n.Scope)
		if err != nil {
			return err
		}
		if n.Super != nil {
			if err := c.expr(e, n.Super); err != nil {
				return err
			}
			flags |= uint64(cameopb.Flag_FLAG_SUPER)
		}
		index, err := c.proc("<class:"+n.Name+">", nil, n.Body, line, false)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_CLASS, line, c.nameArg(n.Name), procArg(index), c.nameArg(n.Digest)).Flags = flags
	case *syntax.ModuleDef:
		flags, err := c.scope(e, n.Scope)
		if err != nil {
			return err
		}
		index, err := c.proc("<module:"+n.Name+">", nil, n.Body, line, false)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_MODULE, line, c.nameArg(n.Name), procArg(index), c.nameArg(n.Digest)).Flags = flags
	case *syntax.Alias:
		e.emit(cameopb.Op_ALIAS, line, c.nameArg(n.New), c.nameArg(n.Old))
	case *syntax.FieldDecl:
		var flags uint64
		if n.Type == "ref" {
			if err := c.expr(e, n.Ref); err != nil {
				return err
			}
			flags |= uint64(cameopb.Flag_FLAG_REF)
		}
		if n.Default != nil {
			if err := c.expr(e, n.Default); err != nil {
				return err
			}
			flags |= uint64(cameopb.Flag_FLAG_DEFAULT)
		}
		decl := &cameopb.FieldDecl{Type: n.Type, Names: n.Names, Number: n.Number, Of: n.Of, Key: n.Key, Value: n.Value, Values: n.Values}
		e.emit(cameopb.Op_FIELD, line, &cameopb.Arg{Kind: &cameopb.Arg_Field{Field: decl}}).Flags = flags
	case *syntax.AttrDecl:
		var flags uint64
		if n.Default != nil {
			if err := c.expr(e, n.Default); err != nil {
				return err
			}
			flags = uint64(cameopb.Flag_FLAG_DEFAULT)
		}
		var args []*cameopb.Arg
		for _, name := range n.Names {
			args = append(args, c.nameArg(name))
		}
		e.emit(cameopb.Op_ATTR, line, args...).Flags = flags
	case *syntax.Reserved:
		e.emit(cameopb.Op_RESERVED, line, &cameopb.Arg{Kind: &cameopb.Arg_Int{Int: n.Number}}, c.nameArg(n.Name))
	case *syntax.Include:
		if err := c.expr(e, n.Module); err != nil {
			return err
		}
		e.emit(cameopb.Op_INCLUDE, line).Flags = flag(cameopb.Flag_FLAG_EXTEND, n.Extend)
	default:
		return &Unsupported{Node: n}
	}
	return nil
}

// scope compiles the module of `class A::B` or `module A::B`, when the
// statement names one, and returns the flag that says it is on the stack.
func (c *compiler) scope(e *emitter, scope syntax.Node) (uint64, error) {
	if scope == nil {
		return 0, nil
	}
	if err := c.expr(e, scope); err != nil {
		return 0, err
	}
	return uint64(cameopb.Flag_FLAG_SCOPED), nil
}

// needCameo emits the check that Cameo is included, which a form written
// Cameo.async does not need. It comes before the form's operands, as the
// check comes before they are evaluated.
func (c *compiler) needCameo(e *emitter, name string, qualified bool, line int) {
	if !qualified {
		e.emit(cameopb.Op_NEED_CAMEO, line, c.nameArg(name))
	}
}

// asyncCall compiles `async recv, :method, *args` and the same form of
// await: the receiver, the method's name and the arguments, then op.
func (c *compiler) asyncCall(e *emitter, op cameopb.Op, name string, qualified bool, recv, method syntax.Node, args []syntax.Node, kwargs []syntax.MapEntry, line int) error {
	c.needCameo(e, name, qualified, line)
	if err := c.expr(e, recv); err != nil {
		return err
	}
	if err := c.expr(e, method); err != nil {
		return err
	}
	argc, flags, err := c.callArgs(e, args, kwargs, line)
	if err != nil {
		return err
	}
	e.emit(op, line, intArg(argc)).Flags = flags | flag(cameopb.Flag_FLAG_QUALIFIED, qualified)
	return nil
}

// super compiles a call of super: a bare one passes the method's own
// arguments, and any super the method's block unless it names another.
func (c *compiler) super(e *emitter, n *syntax.Super) error {
	line := n.Line()
	e.emit(cameopb.Op_NEED_METHOD, line)
	argc, flags := 0, uint64(cameopb.Flag_FLAG_FORWARD)
	if !n.Forward {
		var err error
		if argc, flags, err = c.callArgs(e, n.Args, n.KwArgs, line); err != nil {
			return err
		}
	}
	args := []*cameopb.Arg{intArg(argc)}
	switch {
	case n.Block != nil:
		index, err := c.block(n.Block)
		if err != nil {
			return err
		}
		args = append(args, procArg(index))
		flags |= uint64(cameopb.Flag_FLAG_BLOCK)
	case n.BlockArg != nil:
		if err := c.expr(e, n.BlockArg); err != nil {
			return err
		}
		flags |= uint64(cameopb.Flag_FLAG_BLOCK_ARG)
	}
	e.emit(cameopb.Op_SUPER, line, args...).Flags = flags
	return nil
}

// begin compiles begin/rescue/else/ensure. The lists are the body, the
// else and the ensure, then two per rescue clause: its classes, which
// leaves them as a list and is empty for a clause that names none, and
// its body.
func (c *compiler) begin(e *emitter, n *syntax.Begin) error {
	line := n.Line()
	body, err := c.body(n.Body, line)
	if err != nil {
		return err
	}
	els, err := c.body(n.Else, line)
	if err != nil {
		return err
	}
	ensure, err := c.body(n.Ensure, line)
	if err != nil {
		return err
	}
	code := &cameopb.Code{Op: cameopb.Op_BEGIN, Line: int32(line), Code: []*cameopb.Seq{body, els, ensure}}
	code.Flags = flag(cameopb.Flag_FLAG_ELSE, n.Else != nil) | flag(cameopb.Flag_FLAG_ENSURE, n.Ensure != nil)
	for i := range n.Rescues {
		r := &n.Rescues[i]
		classes := &emitter{}
		if len(r.Classes) > 0 {
			if _, _, err := c.args(classes, r.Classes, line, true); err != nil {
				return err
			}
		}
		clause, err := c.body(r.Body, line)
		if err != nil {
			return err
		}
		code.Args = append(code.Args, c.nameArg(r.Var))
		code.Code = append(code.Code, classes.seq(), clause)
	}
	e.code = append(e.code, code)
	return nil
}

// caseWhen compiles case/when: two lists per when, its conditions as a
// list and its body, then the else.
func (c *compiler) caseWhen(e *emitter, n *syntax.Case) error {
	line := n.Line()
	flags := uint64(cameopb.Flag_FLAG_ELSE)
	if n.Subject != nil {
		if err := c.expr(e, n.Subject); err != nil {
			return err
		}
		flags |= uint64(cameopb.Flag_FLAG_SUBJECT)
	}
	code := &cameopb.Code{Op: cameopb.Op_CASE, Line: int32(line), Flags: flags}
	for _, w := range n.Whens {
		conds := &emitter{}
		if _, _, err := c.args(conds, w.Conds, line, true); err != nil {
			return err
		}
		body, err := c.body(w.Body, line)
		if err != nil {
			return err
		}
		code.Code = append(code.Code, conds.seq(), body)
	}
	els, err := c.body(n.Else, line)
	if err != nil {
		return err
	}
	code.Code = append(code.Code, els)
	e.code = append(e.code, code)
	return nil
}

// caseIn compiles case/in: per clause a pattern, the kind of its guard
// (0 none, 1 if, 2 unless) and two lists, the guard and the body.
func (c *compiler) caseIn(e *emitter, n *syntax.CaseIn) error {
	line := n.Line()
	if err := c.expr(e, n.Subject); err != nil {
		return err
	}
	code := &cameopb.Code{Op: cameopb.Op_CASE_IN, Line: int32(line)}
	for _, cl := range n.Clauses {
		p, err := c.pattern(cl.Pattern)
		if err != nil {
			return err
		}
		guard, kind := &cameopb.Seq{}, 0
		if cl.Guard != nil {
			if guard, err = c.value(cl.Guard); err != nil {
				return err
			}
			kind = 1
			if cl.GuardNeg {
				kind = 2
			}
		}
		body, err := c.body(cl.Body, line)
		if err != nil {
			return err
		}
		code.Args = append(code.Args, &cameopb.Arg{Kind: &cameopb.Arg_Pattern{Pattern: p}}, intArg(kind))
		code.Code = append(code.Code, guard, body)
	}
	if n.Else != nil {
		els, err := c.body(n.Else, line)
		if err != nil {
			return err
		}
		code.Code = append(code.Code, els)
		code.Flags = uint64(cameopb.Flag_FLAG_ELSE)
	}
	e.code = append(e.code, code)
	return nil
}

// defined compiles defined?(expr), which evaluates no more of expr than
// it must: what expr is decides the instruction.
func (c *compiler) defined(e *emitter, n *syntax.Defined) error {
	line := n.Line()
	text := func(s string) {
		e.emit(cameopb.Op_CONST, line, constArg(&cameopb.Constant{Kind: &cameopb.Constant_StringValue{StringValue: s}}))
	}
	switch x := n.Expr.(type) {
	case *syntax.Var:
		e.emit(cameopb.Op_DEFINED_LOCAL, line, c.nameArg(x.Name))
	case *syntax.Const, *syntax.ScopedConst:
		expr, err := c.value(x)
		if err != nil {
			return err
		}
		e.emit(cameopb.Op_DEFINED_CONST, line).Code = []*cameopb.Seq{expr}
	case *syntax.Call:
		recv := &cameopb.Seq{}
		if x.Recv != nil {
			var err error
			if recv, err = c.value(x.Recv); err != nil {
				return err
			}
		}
		e.emit(cameopb.Op_DEFINED_METHOD, line, c.nameArg(x.Name)).Code = []*cameopb.Seq{recv}
	case *syntax.Yield:
		e.emit(cameopb.Op_DEFINED_YIELD, line)
	case *syntax.Self:
		text("self")
	case *syntax.Assign, *syntax.FieldAssign, *syntax.ConstAssign, *syntax.MultiAssign:
		text("assignment")
	default:
		text("expression")
	}
	return nil
}

// pattern compiles a pattern of case/in, `=>` and `in`.
func (c *compiler) pattern(p syntax.Pattern) (*cameopb.Pattern, error) {
	patterns := func(ps []syntax.Pattern) ([]*cameopb.Pattern, error) {
		var out []*cameopb.Pattern
		for _, p := range ps {
			cp, err := c.pattern(p)
			if err != nil {
				return nil, err
			}
			out = append(out, cp)
		}
		return out, nil
	}
	// constOf compiles the constant of `Const[…]` and `Const(…)`, nil
	// when the pattern has none.
	constOf := func(n syntax.Node) (*cameopb.Seq, error) {
		if n == nil {
			return nil, nil
		}
		return c.value(n)
	}
	// named is the index of a name that may be absent.
	named := func(name string) *uint32 {
		if name == "" {
			return nil
		}
		i := c.name(name)
		return &i
	}
	switch p := p.(type) {
	case *syntax.ValuePattern:
		v, err := c.value(p.Value)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Value{Value: v}}, nil
	case *syntax.BindPattern:
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Bind{Bind: &cameopb.Pattern_Local{Name: c.name(p.Name)}}}, nil
	case *syntax.ConstPattern:
		v, err := c.value(p.Const)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Const{Const: v}}, nil
	case *syntax.BindingPattern:
		sub, err := c.pattern(p.Sub)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Binding{Binding: &cameopb.Pattern_As{Sub: sub, Name: c.name(p.Name)}}}, nil
	case *syntax.AltPattern:
		alts, err := patterns(p.Alts)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Alt{Alt: &cameopb.Pattern_Either{Alts: alts}}}, nil
	case *syntax.ListPattern:
		cn, err := constOf(p.Const)
		if err != nil {
			return nil, err
		}
		pre, err := patterns(p.Pre)
		if err != nil {
			return nil, err
		}
		post, err := patterns(p.Post)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_List{List: &cameopb.Pattern_Array{
			Const: cn, Pre: pre, HasSplat: p.HasSplat, SplatName: named(p.SplatName), Post: post,
		}}}, nil
	case *syntax.FindPattern:
		cn, err := constOf(p.Const)
		if err != nil {
			return nil, err
		}
		mid, err := patterns(p.Mid)
		if err != nil {
			return nil, err
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Find{Find: &cameopb.Pattern_Search{
			Const: cn, PreName: named(p.PreName), Mid: mid, PostName: named(p.PostName),
		}}}, nil
	case *syntax.MapPattern:
		cn, err := constOf(p.Const)
		if err != nil {
			return nil, err
		}
		out := &cameopb.Pattern_Hash{Const: cn, HasRest: p.HasRest, RestName: named(p.RestName), RestNil: p.RestNil}
		for i, key := range p.Keys {
			entry := &cameopb.Pattern_Hash_Entry{Key: c.name(key)}
			if p.Values[i] != nil {
				if entry.Value, err = c.pattern(p.Values[i]); err != nil {
					return nil, err
				}
			}
			out.Entries = append(out.Entries, entry)
		}
		return &cameopb.Pattern{Kind: &cameopb.Pattern_Map{Map: out}}, nil
	}
	return nil, fmt.Errorf("a pattern of kind %T does not compile to the IR", p)
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
	keys := 0
	for _, entry := range entries {
		if entry.Key != nil {
			keys++
		}
	}
	// MAP's argument is the room the map makes for its keys.
	e.emit(cameopb.Op_MAP, line, intArg(keys))
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
		e.emit(binaryOps[n.Name], line)
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
