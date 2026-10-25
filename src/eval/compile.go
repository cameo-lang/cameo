package eval

import (
	"math"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A thunk is a compiled node: the Go function that evaluates it. The
// interpreter is a parameter, not a captured variable, so one compiled
// program serves the main interpreter and every async worker alike.
type thunk func(in *Interp, env *Env, fr *Frame) (object.Value, error)

// code is the compiled form of a program, built node by node the first
// time a node is evaluated and shared by every interpreter running the
// program. Nodes the compiler specialises are compiled with their
// children inline; the rest fall back to the tree walker, whose children
// come through the cache.
type code struct {
	exprs  sync.Map // syntax.Node -> thunk
	bodies sync.Map // *syntax.Def or *syntax.Block -> thunk of its body
	// compiled is the Go implementation `cameo build` made of the
	// program's top-level methods, nil when it made none; see
	// RegisterCompiled.
	compiled Compiled
	// debug is set while a debugger is on: every statement compiled from
	// then on reports its line to it (statement). Workers compile on
	// their own goroutines, and require "debug" sets it on the main one.
	debug atomic.Bool
}

// expr returns the thunk of n, compiling it on first use.
func (c *code) expr(n syntax.Node) thunk {
	if t, ok := c.exprs.Load(n); ok {
		return t.(thunk)
	}
	t := c.compile(n)
	c.exprs.Store(n, t)
	return t
}

// body returns the thunk of the body of a def or a block, compiling it on
// first use.
func (c *code) body(owner syntax.Node, body []syntax.Node) thunk {
	if t, ok := c.bodies.Load(owner); ok {
		return t.(thunk)
	}
	t := c.compileBody(body)
	c.bodies.Store(owner, t)
	return t
}

// compiledDef is what a user method's Body holds: its definition and its
// compiled body.
type compiledDef struct {
	def  *syntax.Def
	body thunk
}

// compileBody compiles a statement sequence: the last value, nil for none.
func (c *code) compileBody(body []syntax.Node) thunk {
	switch len(body) {
	case 0:
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	case 1:
		return c.statement(body[0])
	}
	thunks := make([]thunk, len(body))
	for i, n := range body {
		thunks[i] = c.statement(n)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		var last object.Value = object.Nil
		for _, t := range thunks {
			v, err := t(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			last = v
		}
		return last, nil
	}
}

// statement compiles one statement of a body: with the debugger on, its
// thunk tells the debugger the line it is about to run (debug.go).
func (c *code) statement(n syntax.Node) thunk {
	t := c.compile(n)
	if !c.debug.Load() {
		return t
	}
	line := n.Line()
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		if in.dbg != nil {
			if err := in.dbg.line(line, env, fr); err != nil {
				return object.Value{}, err
			}
		}
		return t(in, env, fr)
	}
}

// compileOptional compiles a node that may be absent, reading nil for it.
func (c *code) compileOptional(n syntax.Node) thunk {
	if n == nil {
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	}
	return c.compile(n)
}

// compile turns one node into its thunk. The kinds that carry the cost of
// a running program are specialised here; every other kind is evaluated
// by the tree walker in evalSlow.
func (c *code) compile(n syntax.Node) thunk {
	switch n := n.(type) {
	case *syntax.Nil:
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	case *syntax.Bool:
		v := object.Bool(n.Value)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Int:
		v := object.Int(n.Value)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.BigInt:
		b, _ := new(big.Int).SetString(n.Digits, 10)
		v := object.BigInt(b)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Float:
		v := object.Float(n.Value)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Str:
		v := object.String(n.Value)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Sym:
		v := object.Symbol(n.Name)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Self:
		return func(_ *Interp, _ *Env, fr *Frame) (object.Value, error) { return fr.Self, nil }
	case *syntax.Var:
		name := n.Name
		return func(_ *Interp, env *Env, _ *Frame) (object.Value, error) {
			if v, ok := env.Get(name); ok {
				return v, nil
			}
			return object.Nil, nil
		}
	case *syntax.Assign:
		name, value := n.Name, c.compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			env.Set(name, v)
			return v, nil
		}
	case *syntax.FieldAssign:
		name, line, value := n.Name, n.Line(), c.compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			return v, in.writeField(name, v, fr, line)
		}
	case *syntax.Regexp:
		return c.compileRegexp(n)
	case *syntax.Const:
		name, line := n.Name, n.Line()
		return func(in *Interp, _ *Env, fr *Frame) (object.Value, error) {
			return in.lookupConst(name, fr, line)
		}
	case *syntax.Interp:
		parts := make([]thunk, len(n.Parts))
		for i, p := range n.Parts {
			parts[i] = c.compile(p)
		}
		line := n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			var b strings.Builder
			// The result takes the encoding its parts are compatible in, as
			// Ruby's does: a binary part in UTF-8 text makes it binary.
			// UTF-8 parts alone, the usual case, need no looking at.
			kind := object.KindString
			for _, part := range parts {
				v, err := part(in, env, fr)
				if err != nil {
					return object.Value{}, err
				}
				s, err := in.ToS(v)
				if err != nil {
					return object.Value{}, withLine(err, line)
				}
				if pk := v.Kind(); v.IsString() && (pk != kind || kind != object.KindString) {
					sofar := b.String()
					next, ok := stdlib.CombineEncodings(kind, sofar == "", object.ASCIIOnly(sofar), pk, s == "", object.ASCIIOnly(s))
					if !ok {
						return object.Value{}, in.raise(object.CompatibilityError, line, "%s", stdlib.IncompatibleEncodings(kind, pk))
					}
					kind = next
				}
				b.WriteString(s)
			}
			return object.StringOf(kind, b.String()), nil
		}
	case *syntax.List:
		if elems, ok := c.compileArgs(n.Elems); ok {
			return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
				values, err := elems(in, env, fr)
				if err != nil {
					return object.Value{}, err
				}
				return object.NewList(values...), nil
			}
		}
	case *syntax.Call:
		return c.compileCall(n)
	case *syntax.Lambda:
		block, strict := n.Block, n.Strict
		return func(_ *Interp, env *Env, fr *Frame) (object.Value, error) {
			return procValue(&Proc{Block: block, Env: env, Frame: fr, Strict: strict}), nil
		}
	case *syntax.Yield:
		args, ok := c.compileArgs(n.Args)
		if !ok || len(n.KwArgs) > 0 {
			break
		}
		line := n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			if fr.Block.IsZero() {
				return object.Value{}, in.raise(object.LocalJumpError, line, "no block given (yield)")
			}
			values, err := args(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			v, err := in.callBlock(fr.Block, values, nil, object.Value{})
			return v, withLine(err, line)
		}
	case *syntax.Return:
		value, line := c.compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, &control{kind: ctrlReturn, value: v, target: fr, line: line}
		}
	case *syntax.Break:
		value, line := c.compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, &control{kind: ctrlBreak, value: v, line: line}
		}
	case *syntax.Next:
		value, line := c.compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, &control{kind: ctrlNext, value: v, line: line}
		}
	case *syntax.Redo:
		line := n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			return object.Value{}, &control{kind: ctrlRedo, line: line}
		}
	case *syntax.If:
		cond, then, els := c.compile(n.Cond), c.compileBody(n.Then), c.compileBody(n.Else)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			c, err := cond(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			if c.Truthy() {
				return then(in, env, fr)
			}
			return els(in, env, fr)
		}
	case *syntax.While:
		cond, body, test := c.compile(n.Cond), c.compileBody(n.Body), !n.DoWhile
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			for first := true; ; first = false {
				if err := in.stopped(); err != nil {
					return object.Value{}, err
				}
				if test || !first {
					c, err := cond(in, env, fr)
					if err != nil {
						return object.Value{}, err
					}
					if !c.Truthy() {
						return object.Nil, nil
					}
				}
			pass:
				if _, err := body(in, env, fr); err != nil {
					if c, ok := err.(*control); ok && c.target == nil {
						switch c.kind {
						case ctrlBreak:
							return c.value, nil
						case ctrlNext:
							continue
						case ctrlRedo:
							if err := in.stopped(); err != nil {
								return object.Value{}, err
							}
							goto pass
						}
					}
					return object.Value{}, err
				}
			}
		}
	case *syntax.And:
		left, right := c.compile(n.Left), c.compile(n.Right)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			l, err := left(in, env, fr)
			if err != nil || !l.Truthy() {
				return l, err
			}
			return right(in, env, fr)
		}
	case *syntax.Or:
		left, right := c.compile(n.Left), c.compile(n.Right)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			l, err := left(in, env, fr)
			if err != nil || l.Truthy() {
				return l, err
			}
			return right(in, env, fr)
		}
	case *syntax.Not:
		value := c.compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			return object.Bool(!v.Truthy()), nil
		}
	case *syntax.Begin:
		return c.compileBegin(n)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		return in.evalSlow(n, env, fr)
	}
}

// argsThunk evaluates an argument list into a fresh slice.
type argsThunk func(in *Interp, env *Env, fr *Frame) ([]object.Value, error)

// compileArgs compiles a list of expressions with no splat; false when
// there is one, which the tree walker splices.
func (c *code) compileArgs(nodes []syntax.Node) (argsThunk, bool) {
	thunks := make([]thunk, len(nodes))
	for i, n := range nodes {
		if _, splat := n.(*syntax.Splat); splat {
			return nil, false
		}
		thunks[i] = c.compile(n)
	}
	switch len(thunks) {
	case 0:
		return func(*Interp, *Env, *Frame) ([]object.Value, error) { return nil, nil }, true
	case 1:
		one := thunks[0]
		return func(in *Interp, env *Env, fr *Frame) ([]object.Value, error) {
			v, err := one(in, env, fr)
			if err != nil {
				return nil, err
			}
			return []object.Value{v}, nil
		}, true
	}
	return func(in *Interp, env *Env, fr *Frame) ([]object.Value, error) {
		out := make([]object.Value, len(thunks))
		for i, t := range thunks {
			v, err := t(in, env, fr)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}, true
}

// compileCall compiles a method call. A call with a splat, keyword
// arguments or a &block argument is left to the tree walker.
func (c *code) compileCall(n *syntax.Call) thunk {
	line, name, safe, vcall := n.Line(), n.Name, n.Safe, n.VCall
	selfCall := n.Recv == nil
	if selfCall && name == "block_given?" && len(n.Args) == 0 {
		return func(_ *Interp, _ *Env, fr *Frame) (object.Value, error) {
			return object.Bool(!fr.Block.IsZero()), nil
		}
	}
	args, simple := c.compileArgs(n.Args)
	if !simple || len(n.KwArgs) > 0 || n.BlockArg != nil {
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			return in.evalCall(n, env, fr)
		}
	}
	var recv thunk
	if !selfCall {
		recv = c.compile(n.Recv)
	}
	block := n.Block
	if t := c.compileBinary(n); t != nil {
		return t
	}
	checkMutation := !selfCall && mayMutate(name)
	recvName := syntax.RecvName(n.Recv)
	dropped := !selfCall && n.Dropped
	rebinds := n.Rebinds
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		r := fr.Self
		if recv != nil {
			v, err := recv(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			r = v
			if safe && r.IsNil() {
				return object.Nil, nil
			}
		}
		values, err := args(in, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		var blk object.Value
		if block != nil {
			blk = procValue(&Proc{Block: block, Env: env, Frame: fr})
		}
		if checkMutation {
			if v, handled, err := in.mutating(fr, r, recvName, name, values, nil, blk, rebinds, line); handled {
				return v, err
			}
		}
		v, err := in.dispatch(fr, r, name, values, nil, blk, selfCall, vcall, line)
		if err != nil {
			return object.Value{}, withLine(err, line)
		}
		if dropped {
			if err := in.dropped(r, name, line); err != nil {
				return object.Value{}, err
			}
		}
		return v, nil
	}
}

// compileBegin compiles begin/rescue/else/ensure. The rescue clauses'
// classes are evaluated by the tree walker when an exception arrives.
func (c *code) compileBegin(n *syntax.Begin) thunk {
	body, els := c.compileBody(n.Body), c.compileBody(n.Else)
	hasElse := n.Else != nil
	clauses := make([]thunk, len(n.Rescues))
	for i := range n.Rescues {
		clauses[i] = c.compileBody(n.Rescues[i].Body)
	}
	var ensure thunk
	if n.Ensure != nil {
		ensure = c.compileBody(n.Ensure)
	}
	run := func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		for {
			result, err := body(in, env, fr)
			if err == nil {
				if hasElse {
					return els(in, env, fr)
				}
				return result, nil
			}
			exc, ok := err.(*object.Exception)
			if !ok {
				return object.Value{}, err
			}
			i, err := in.matchRescue(n.Rescues, exc, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			if i < 0 {
				return object.Value{}, exc
			}
			if in.rescuing != nil {
				// Raised inside another rescue clause and rescued before it
				// ends: it takes its cause where it is caught.
				exc = caused(exc, in.rescuing).(*object.Exception)
			}
			if v := n.Rescues[i].Var; v != "" {
				env.Set(v, object.ExceptionValue(exc))
			}
			outer := in.rescuing
			in.rescuing = exc
			result, err = clauses[i](in, env, fr)
			in.rescuing = outer
			if err == nil {
				return result, nil
			}
			if c, ok := err.(*control); ok && c.kind == ctrlRetry {
				continue
			}
			return result, caused(err, exc)
		}
	}
	if ensure == nil {
		return run
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		result, err := run(in, env, fr)
		if _, ensureErr := ensure(in, env, fr); ensureErr != nil {
			return object.Value{}, ensureErr
		}
		return result, err
	}
}

// Operators on integers.
//
// An intThunk evaluates an expression and returns an integer result as a
// raw int64: v is the zero Value and i holds the integer. Any other
// result, nil included, comes back as v. An arithmetic expression over
// integers compiles to a tree of these, so its intermediate results skip
// the Value and its kind checks; only the value the tree is finally used
// as is made.
type intThunk func(in *Interp, env *Env, fr *Frame) (i int64, v object.Value, err error)

// compileInt compiles n for an operator that wants an integer operand.
func (c *code) compileInt(n syntax.Node) intThunk {
	switch n := n.(type) {
	case *syntax.Int:
		c := n.Value
		return func(*Interp, *Env, *Frame) (int64, object.Value, error) { return c, object.Value{}, nil }
	case *syntax.Var:
		name := n.Name
		return func(_ *Interp, env *Env, _ *Frame) (int64, object.Value, error) {
			v, ok := env.Get(name)
			if !ok {
				return 0, object.Nil, nil
			}
			if i, ok := v.AsInt(); ok {
				return int64(i), object.Value{}, nil
			}
			return 0, v, nil
		}
	case *syntax.Call:
		if operatorCall(n) {
			if t := c.compileIntArith(n); t != nil {
				return t
			}
		}
	}
	t := c.compile(n)
	return func(in *Interp, env *Env, fr *Frame) (int64, object.Value, error) {
		v, err := t(in, env, fr)
		if err != nil {
			return 0, object.Value{}, err
		}
		if i, ok := v.AsInt(); ok {
			return int64(i), object.Value{}, nil
		}
		return 0, v, nil
	}
}

// intArith returns the integer arithmetic for an operator name, or nil.
// ok is false when the operands need the ordinary call: a zero divisor, or
// a result beyond 64 bits, which the ordinary call makes a big integer.
func intArith(name string) func(x, y int64) (r int64, ok bool) {
	switch name {
	case "+":
		return object.AddInt
	case "-":
		return object.SubInt
	case "*":
		return object.MulInt
	case "/":
		return func(x, y int64) (int64, bool) {
			if y == 0 || (y == -1 && x == math.MinInt64) {
				return 0, false
			}
			q := x / y
			if x%y != 0 && (x < 0) != (y < 0) {
				q--
			}
			return q, true
		}
	case "%":
		return func(x, y int64) (int64, bool) {
			if y == 0 {
				return 0, false
			}
			m := x % y
			if m != 0 && (m < 0) != (y < 0) {
				m += y
			}
			return m, true
		}
	}
	return nil
}

// intCompare returns the integer comparison for an operator name, or nil.
func intCompare(name string) func(x, y int64) bool {
	switch name {
	case "<":
		return func(x, y int64) bool { return x < y }
	case "<=":
		return func(x, y int64) bool { return x <= y }
	case ">":
		return func(x, y int64) bool { return x > y }
	case ">=":
		return func(x, y int64) bool { return x >= y }
	case "==":
		return func(x, y int64) bool { return x == y }
	case "!=":
		return func(x, y int64) bool { return x != y }
	}
	return nil
}

// operatorCall reports whether n has the shape the operator fast paths
// take: a receiver, one plain argument, nothing else. A safe call `&.`
// is left to the ordinary path.
func operatorCall(n *syntax.Call) bool {
	if n.Recv == nil || len(n.Args) != 1 || n.Block != nil || n.BlockArg != nil || len(n.KwArgs) > 0 || n.Safe {
		return false
	}
	_, splat := n.Args[0].(*syntax.Splat)
	return !splat
}

// operands compiles the two operands of an operator call and evaluates
// them, left first; both come back as raw integers when both are integers.
type operands func(in *Interp, env *Env, fr *Frame) (x, y int64, xv, yv object.Value, err error)

func (c *code) compileOperands(n *syntax.Call) operands {
	left, right := c.compileInt(n.Recv), c.compileInt(n.Args[0])
	return func(in *Interp, env *Env, fr *Frame) (int64, int64, object.Value, object.Value, error) {
		x, xv, err := left(in, env, fr)
		if err != nil {
			return 0, 0, object.Value{}, object.Value{}, err
		}
		y, yv, err := right(in, env, fr)
		if err != nil {
			return 0, 0, object.Value{}, object.Value{}, err
		}
		return x, y, xv, yv, nil
	}
}

// dispatch is the ordinary call for operands the fast path does not
// cover: makes Values of the raw integers, tries the Value fast path (two
// floats), then dispatches.
func dispatch(in *Interp, fr *Frame, name string, op func(a, b object.Value) object.Value, line int, x, y int64, xv, yv object.Value) (object.Value, error) {
	a, b := xv, yv
	if a.IsZero() {
		a = object.Int(x)
	}
	if b.IsZero() {
		b = object.Int(y)
	}
	if v := op(a, b); !v.IsZero() {
		return v, nil
	}
	v, err := in.call(fr, a, name, []object.Value{b}, nil, object.Value{}, false, line)
	return v, withLine(err, line)
}

// compileIntArith compiles an arithmetic operator call to an intThunk,
// or nil when the name is not one.
func (c *code) compileIntArith(n *syntax.Call) intThunk {
	arith := intArith(n.Name)
	if arith == nil {
		return nil
	}
	ops, op, name, line := c.compileOperands(n), binaryOp(n.Name), n.Name, n.Line()
	return func(in *Interp, env *Env, fr *Frame) (int64, object.Value, error) {
		x, y, xv, yv, err := ops(in, env, fr)
		if err != nil {
			return 0, object.Value{}, err
		}
		if xv.IsZero() && yv.IsZero() {
			if r, ok := arith(x, y); ok {
				return r, object.Value{}, nil
			}
		}
		v, err := dispatch(in, fr, name, op, line, x, y, xv, yv)
		return 0, v, err
	}
}

// compileBinary compiles an operator call with one argument to a thunk,
// or nil when the operator has no fast path. Arithmetic keeps its
// operands' intermediate results as raw integers; a comparison reads them
// the same way and returns a boolean.
func (c *code) compileBinary(n *syntax.Call) thunk {
	if !operatorCall(n) {
		return nil
	}
	if arith := c.compileIntArith(n); arith != nil {
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			i, v, err := arith(in, env, fr)
			if err != nil || !v.IsZero() {
				return v, err
			}
			return object.Int(i), nil
		}
	}
	cmp := intCompare(n.Name)
	if cmp == nil {
		return nil
	}
	ops, op, name, line := c.compileOperands(n), binaryOp(n.Name), n.Name, n.Line()
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		x, y, xv, yv, err := ops(in, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		if xv.IsZero() && yv.IsZero() {
			return object.Bool(cmp(x, y)), nil
		}
		return dispatch(in, fr, name, op, line, x, y, xv, yv)
	}
}

// compileRegexp compiles a regexp literal: once, on first use, when it
// has no interpolation, and on every evaluation otherwise, where a regexp
// interpolated embeds as its to_s, (?flags:source), as in Ruby.
func (c *code) compileRegexp(n *syntax.Regexp) thunk {
	line, options := n.Line(), n.Options
	build := func(in *Interp, source string) (object.Value, error) {
		re, err := object.NewRegexp(source, options)
		if err != nil {
			return object.Value{}, withLine(stdlib.RegexpFailure(in, err, source), line)
		}
		return object.RegexpValue(re), nil
	}
	if len(n.Parts) == 1 {
		if s, ok := n.Parts[0].(*syntax.Str); ok {
			var once sync.Once
			var cached object.Value
			var cacheErr error
			return func(in *Interp, _ *Env, _ *Frame) (object.Value, error) {
				once.Do(func() { cached, cacheErr = build(in, s.Value) })
				return cached, cacheErr
			}
		}
	}
	parts := make([]thunk, len(n.Parts))
	for i, p := range n.Parts {
		parts[i] = c.compile(p)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		var b strings.Builder
		for _, p := range parts {
			v, err := p(in, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			if re, ok := v.AsRegexp(); ok {
				b.WriteString(re.String())
				continue
			}
			s, err := in.ToS(v)
			if err != nil {
				return object.Value{}, err
			}
			b.WriteString(s)
		}
		return build(in, b.String())
	}
}
