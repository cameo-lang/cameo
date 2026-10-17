package eval

import (
	"strings"
	"sync"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
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
}

// expr answers the thunk of n, compiling it on first use.
func (c *code) expr(n syntax.Node) thunk {
	if t, ok := c.exprs.Load(n); ok {
		return t.(thunk)
	}
	t := compile(n)
	c.exprs.Store(n, t)
	return t
}

// body answers the thunk of the body of a def or a block, compiling it on
// first use.
func (c *code) body(owner syntax.Node, body []syntax.Node) thunk {
	if t, ok := c.bodies.Load(owner); ok {
		return t.(thunk)
	}
	t := compileBody(body)
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
func compileBody(body []syntax.Node) thunk {
	switch len(body) {
	case 0:
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	case 1:
		return compile(body[0])
	}
	thunks := make([]thunk, len(body))
	for i, n := range body {
		thunks[i] = compile(n)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		var last object.Value = object.Nil
		for _, t := range thunks {
			v, err := t(in, env, fr)
			if err != nil {
				return nil, err
			}
			last = v
		}
		return last, nil
	}
}

// compileOptional compiles a node that may be absent, reading nil for it.
func compileOptional(n syntax.Node) thunk {
	if n == nil {
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	}
	return compile(n)
}

// compile turns one node into its thunk. The kinds that carry the cost of
// a running program are specialised here; every other kind is evaluated
// by the tree walker in evalSlow.
func compile(n syntax.Node) thunk {
	switch n := n.(type) {
	case *syntax.Nil:
		return func(*Interp, *Env, *Frame) (object.Value, error) { return object.Nil, nil }
	case *syntax.Bool:
		v := object.BoolOf(n.Value)
		return func(*Interp, *Env, *Frame) (object.Value, error) { return v, nil }
	case *syntax.Int:
		v := object.IntValue(n.Value)
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
		name, value := n.Name, compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			env.Set(name, v)
			return v, nil
		}
	case *syntax.IVar:
		name, line := n.Name, n.Line()
		return func(in *Interp, _ *Env, fr *Frame) (object.Value, error) {
			return in.readIVar(name, fr, line)
		}
	case *syntax.IVarAssign:
		name, line, value := n.Name, n.Line(), compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			return v, in.writeIVar(name, v, fr, line)
		}
	case *syntax.Const:
		name, line := n.Name, n.Line()
		return func(in *Interp, _ *Env, fr *Frame) (object.Value, error) {
			return in.lookupConst(name, fr, line)
		}
	case *syntax.Interp:
		parts := make([]thunk, len(n.Parts))
		for i, p := range n.Parts {
			parts[i] = compile(p)
		}
		line := n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			var b strings.Builder
			for _, part := range parts {
				v, err := part(in, env, fr)
				if err != nil {
					return nil, err
				}
				s, err := in.ToS(v)
				if err != nil {
					return nil, withLine(err, line)
				}
				b.WriteString(s)
			}
			return object.String(b.String()), nil
		}
	case *syntax.List:
		if elems, ok := compileArgs(n.Elems); ok {
			return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
				values, err := elems(in, env, fr)
				if err != nil {
					return nil, err
				}
				return object.NewList(values...), nil
			}
		}
	case *syntax.Call:
		return compileCall(n)
	case *syntax.Lambda:
		block, strict := n.Block, n.Strict
		return func(_ *Interp, env *Env, fr *Frame) (object.Value, error) {
			return &Proc{Block: block, Env: env, Frame: fr, Strict: strict}, nil
		}
	case *syntax.Yield:
		args, ok := compileArgs(n.Args)
		if !ok || len(n.KwArgs) > 0 {
			break
		}
		line := n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			if fr.Block == nil {
				return nil, in.raise(object.LocalJumpError, line, "no block given (yield)")
			}
			values, err := args(in, env, fr)
			if err != nil {
				return nil, err
			}
			v, err := in.callBlock(fr.Block, values, nil, nil)
			return v, withLine(err, line)
		}
	case *syntax.Return:
		value, line := compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			return nil, &control{kind: ctrlReturn, value: v, target: fr, line: line}
		}
	case *syntax.Break:
		value, line := compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			return nil, &control{kind: ctrlBreak, value: v, line: line}
		}
	case *syntax.Next:
		value, line := compileOptional(n.Value), n.Line()
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			return nil, &control{kind: ctrlNext, value: v, line: line}
		}
	case *syntax.If:
		cond, then, els := compile(n.Cond), compileBody(n.Then), compileBody(n.Else)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			c, err := cond(in, env, fr)
			if err != nil {
				return nil, err
			}
			if object.Truthy(c) {
				return then(in, env, fr)
			}
			return els(in, env, fr)
		}
	case *syntax.While:
		cond, body := compile(n.Cond), compileBody(n.Body)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			for {
				c, err := cond(in, env, fr)
				if err != nil {
					return nil, err
				}
				if !object.Truthy(c) {
					return object.Nil, nil
				}
				if _, err := body(in, env, fr); err != nil {
					if c, ok := err.(*control); ok && c.target == nil {
						switch c.kind {
						case ctrlBreak:
							return c.value, nil
						case ctrlNext:
							continue
						}
					}
					return nil, err
				}
			}
		}
	case *syntax.And:
		left, right := compile(n.Left), compile(n.Right)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			l, err := left(in, env, fr)
			if err != nil || !object.Truthy(l) {
				return l, err
			}
			return right(in, env, fr)
		}
	case *syntax.Or:
		left, right := compile(n.Left), compile(n.Right)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			l, err := left(in, env, fr)
			if err != nil || object.Truthy(l) {
				return l, err
			}
			return right(in, env, fr)
		}
	case *syntax.Not:
		value := compile(n.Value)
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			v, err := value(in, env, fr)
			if err != nil {
				return nil, err
			}
			return object.BoolOf(!object.Truthy(v)), nil
		}
	case *syntax.Begin:
		return compileBegin(n)
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		return in.evalSlow(n, env, fr)
	}
}

// argsThunk evaluates an argument list into a fresh slice.
type argsThunk func(in *Interp, env *Env, fr *Frame) ([]object.Value, error)

// compileArgs compiles a list of expressions with no splat; false when
// there is one, which the tree walker splices.
func compileArgs(nodes []syntax.Node) (argsThunk, bool) {
	thunks := make([]thunk, len(nodes))
	for i, n := range nodes {
		if _, splat := n.(*syntax.Splat); splat {
			return nil, false
		}
		thunks[i] = compile(n)
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
func compileCall(n *syntax.Call) thunk {
	line, name, safe := n.Line(), n.Name, n.Safe
	selfCall := n.Recv == nil
	if selfCall && name == "block_given?" && len(n.Args) == 0 {
		return func(_ *Interp, _ *Env, fr *Frame) (object.Value, error) {
			return object.BoolOf(fr.Block != nil), nil
		}
	}
	args, simple := compileArgs(n.Args)
	if !simple || len(n.KwArgs) > 0 || n.BlockArg != nil {
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			return in.evalCall(n, env, fr)
		}
	}
	var recv thunk
	if !selfCall {
		recv = compile(n.Recv)
	}
	block := n.Block
	if op := binaryOp(name); op != nil && recv != nil && len(n.Args) == 1 && block == nil {
		arg := compile(n.Args[0])
		return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
			r, err := recv(in, env, fr)
			if err != nil {
				return nil, err
			}
			if safe && r == object.Nil {
				return object.Nil, nil
			}
			a, err := arg(in, env, fr)
			if err != nil {
				return nil, err
			}
			if v := op(r, a); v != nil {
				return v, nil
			}
			v, err := in.call(fr, r, name, []object.Value{a}, nil, nil, false, line)
			return v, withLine(err, line)
		}
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		r := fr.Self
		if recv != nil {
			v, err := recv(in, env, fr)
			if err != nil {
				return nil, err
			}
			r = v
			if safe && r == object.Nil {
				return object.Nil, nil
			}
		}
		values, err := args(in, env, fr)
		if err != nil {
			return nil, err
		}
		var blk object.Value
		if block != nil {
			blk = &Proc{Block: block, Env: env, Frame: fr}
		}
		v, err := in.call(fr, r, name, values, nil, blk, selfCall, line)
		if err != nil {
			return nil, withLine(err, line)
		}
		return v, nil
	}
}

// compileBegin compiles begin/rescue/else/ensure. The rescue clauses'
// classes are evaluated by the tree walker when an exception arrives.
func compileBegin(n *syntax.Begin) thunk {
	body, els := compileBody(n.Body), compileBody(n.Else)
	hasElse := n.Else != nil
	clauses := make([]thunk, len(n.Rescues))
	for i := range n.Rescues {
		clauses[i] = compileBody(n.Rescues[i].Body)
	}
	var ensure thunk
	if n.Ensure != nil {
		ensure = compileBody(n.Ensure)
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
				return nil, err
			}
			i, err := in.matchRescue(n.Rescues, exc, env, fr)
			if err != nil {
				return nil, err
			}
			if i < 0 {
				return nil, exc
			}
			if v := n.Rescues[i].Var; v != "" {
				env.Set(v, exc)
			}
			result, err = clauses[i](in, env, fr)
			if c, ok := err.(*control); ok && c.kind == ctrlRetry {
				continue
			}
			return result, err
		}
	}
	if ensure == nil {
		return run
	}
	return func(in *Interp, env *Env, fr *Frame) (object.Value, error) {
		result, err := run(in, env, fr)
		if _, ensureErr := ensure(in, env, fr); ensureErr != nil {
			return nil, ensureErr
		}
		return result, err
	}
}
