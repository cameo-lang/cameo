package eval

import (
	"math"
	"sync"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A thunk is a compiled call: the Go function that evaluates it. The
// interpreter is a parameter, not a captured variable, so one compiled
// program serves the main interpreter and every async worker alike.
type thunk func(in *Interp, env *Env, fr *Frame) (object.Value, error)

// code is what the interpreters of one program share: the thunks of its
// calls, built the first time a call is evaluated, and the values of its
// literals that are made once.
type code struct {
	exprs sync.Map // *syntax.Call -> thunk
	// literals holds the value of a BigInt literal and of a Regexp
	// literal without interpolation, by node.
	literals sync.Map // syntax.Node -> object.Value
	// compiled is the Go implementation `cameo build` made of the
	// program's top-level methods, nil when it made none; see
	// RegisterCompiled.
	compiled Compiled
}

// expr returns the thunk of n, compiling it on first use.
func (c *code) expr(n *syntax.Call) thunk {
	if t, ok := c.exprs.Load(n); ok {
		return t.(thunk)
	}
	t := c.compileCall(n)
	c.exprs.Store(n, t)
	return t
}

// literal returns the value of the literal n, which make makes on first
// use. A literal make fails for is made again on its next evaluation.
func (c *code) literal(n syntax.Node, make func() (object.Value, error)) (object.Value, error) {
	if v, ok := c.literals.Load(n); ok {
		return v.(object.Value), nil
	}
	v, err := make()
	if err != nil {
		return object.Value{}, err
	}
	actual, _ := c.literals.LoadOrStore(n, v)
	return actual.(object.Value), nil
}

// compile returns the thunk of an operand of a call: the call's own when
// it is one, the tree walker's evaluation of any other node.
func (c *code) compile(n syntax.Node) thunk {
	if call, ok := n.(*syntax.Call); ok {
		return c.compileCall(call)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		return in.eval(n, env, fr)
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
	private := selfCall || isSelf(n.Recv)
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
			blk = procValue(&Proc{Block: block, Env: blockEnv(block, env), Frame: fr})
		}
		if checkMutation {
			if v, handled, err := in.mutating(fr, r, recvName, name, values, nil, blk, rebinds, line); handled {
				return v, err
			}
		}
		v, err := in.dispatch(fr, r, name, values, nil, blk, selfCall, vcall, private, line)
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
