package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// evalBody evaluates statements in order and returns the last value, nil
// when there are none.
func (in *Interp) evalBody(body []syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	var last object.Value = object.Nil
	for _, n := range body {
		v, err := in.eval(n, env, fr)
		if err != nil {
			return nil, err
		}
		last = v
	}
	return last, nil
}

// eval evaluates one node through its compiled thunk.
func (in *Interp) eval(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	return in.code.expr(n)(in, env, fr)
}

// evalSlow is the tree walker: it evaluates the kinds of node the compiler
// does not specialise, and the cases the compiler leaves to it (a list or
// a yield with a splat, a call with keywords). Children are evaluated
// through eval, so they are compiled.
func (in *Interp) evalSlow(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	switch n := n.(type) {
	case *syntax.List:
		elems, err := in.evalArgs(n.Elems, env, fr)
		if err != nil {
			return nil, err
		}
		return object.NewList(elems...), nil
	case *syntax.Map:
		return in.evalMap(n.Entries, env, fr, n.Line())
	case *syntax.Range:
		r := &object.Range{Exclusive: n.Exclusive}
		var err error
		if n.Lo != nil {
			if r.Lo, err = in.eval(n.Lo, env, fr); err != nil {
				return nil, err
			}
		}
		if n.Hi != nil {
			if r.Hi, err = in.eval(n.Hi, env, fr); err != nil {
				return nil, err
			}
		}
		return r, nil
	case *syntax.MultiAssign:
		return in.evalMultiAssign(n, env, fr)
	case *syntax.ScopedConst:
		return in.evalScopedConst(n, env, fr)
	case *syntax.ConstAssign:
		v, err := in.eval(n.Value, env, fr)
		if err != nil {
			return nil, err
		}
		return v, in.defineConst(n.Name, v, fr, n.Line())
	case *syntax.Yield:
		if fr.Block == nil {
			return nil, in.raise(object.LocalJumpError, n.Line(), "no block given (yield)")
		}
		args, kw, err := in.evalCallArgs(n.Args, n.KwArgs, env, fr, n.Line())
		if err != nil {
			return nil, err
		}
		v, err := in.callBlock(fr.Block, args, kw, nil)
		return v, withLine(err, n.Line())
	case *syntax.Super:
		return in.evalSuper(n, env, fr)
	case *syntax.Retry:
		return nil, &control{kind: ctrlRetry, line: n.Line()}
	case *syntax.Case:
		return in.evalCase(n, env, fr)
	case *syntax.CaseIn:
		return in.evalCaseIn(n, env, fr)
	case *syntax.MatchPattern:
		return in.evalMatchPattern(n, env, fr)
	case *syntax.Def:
		return in.evalDef(n, fr)
	case *syntax.ClassDef:
		return in.evalClassDef(n, env, fr)
	case *syntax.ModuleDef:
		return in.evalModuleDef(n, env, fr)
	case *syntax.FieldDecl:
		return in.evalFieldDecl(n, env, fr)
	case *syntax.Reserved:
		return in.evalReserved(n, fr)
	case *syntax.Include:
		return in.evalInclude(n, env, fr)
	case *syntax.With:
		return in.evalWith(n, env, fr)
	case *syntax.Async:
		return in.evalAsync(n, env, fr)
	case *syntax.Await:
		return in.evalAwait(n, env, fr)
	case *syntax.AwaitAll:
		return in.evalAwaitAll(env, n.Line())
	case *syntax.Splat:
		return nil, in.raise(object.AmberError, n.Line(), "a splat belongs in a list or an argument list")
	}
	return nil, in.raise(object.AmberError, n.Line(), "cannot evaluate %T", n)
}

// evalArgs evaluates a list of expressions, splicing splats.
func (in *Interp) evalArgs(nodes []syntax.Node, env *Env, fr *Frame) ([]object.Value, error) {
	out := make([]object.Value, 0, len(nodes))
	for _, n := range nodes {
		if s, ok := n.(*syntax.Splat); ok {
			v, err := in.eval(s.Value, env, fr)
			if err != nil {
				return nil, err
			}
			switch v := v.(type) {
			case *object.List:
				out = append(out, v.Elems...)
			case *object.Set:
				out = append(out, v.Elems()...)
			case *object.Range:
				list, err := in.rangeToList(v, s.Line())
				if err != nil {
					return nil, err
				}
				out = append(out, list.Elems...)
			default:
				if v == object.Nil {
					continue
				}
				out = append(out, v)
			}
			continue
		}
		v, err := in.eval(n, env, fr)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// evalMap builds a map from literal entries, merging double splats.
func (in *Interp) evalMap(entries []syntax.MapEntry, env *Env, fr *Frame, line int) (*object.Map, error) {
	b := object.NewMapBuilder()
	for _, e := range entries {
		v, err := in.eval(e.Value, env, fr)
		if err != nil {
			return nil, err
		}
		if e.Key == nil {
			if v == object.Nil {
				continue
			}
			m, ok := v.(*object.Map)
			if !ok {
				return nil, in.raise(object.TypeError, line, "no implicit conversion of %s into Map", typeName(v))
			}
			for i, k := range m.Keys() {
				b.Set(k, m.Values()[i])
			}
			continue
		}
		k, err := in.eval(e.Key, env, fr)
		if err != nil {
			return nil, err
		}
		b.Set(k, v)
	}
	return b.Map(), nil
}

// evalCallArgs evaluates positional and keyword arguments; kw is nil when
// there are no keywords.
func (in *Interp) evalCallArgs(args []syntax.Node, kwargs []syntax.MapEntry, env *Env, fr *Frame, line int) ([]object.Value, *object.Map, error) {
	values, err := in.evalArgs(args, env, fr)
	if err != nil {
		return nil, nil, err
	}
	if len(kwargs) == 0 {
		return values, nil, nil
	}
	kw, err := in.evalMap(kwargs, env, fr, line)
	if err != nil {
		return nil, nil, err
	}
	return values, kw, nil
}

func (in *Interp) evalMultiAssign(n *syntax.MultiAssign, env *Env, fr *Frame) (object.Value, error) {
	values, err := in.evalArgs(n.Values, env, fr)
	if err != nil {
		return nil, err
	}
	var result object.Value
	if len(n.Values) == 1 {
		result = values[0]
		if l, ok := values[0].(*object.List); ok {
			values = l.Elems
		}
	} else {
		result = object.NewList(values...)
	}
	assignNames(n.Names, n.SplatIndex, values, env)
	return result, nil
}

// assignNames destructures values over names with an optional splat.
func assignNames(names []string, splat int, values []object.Value, env *Env) {
	if splat < 0 {
		for i, name := range names {
			if i < len(values) {
				env.Set(name, values[i])
			} else {
				env.Set(name, object.Nil)
			}
		}
		return
	}
	after := len(names) - splat - 1
	for i := 0; i < splat; i++ {
		env.Set(names[i], at(values, i))
	}
	rest := []object.Value{}
	if len(values) > splat+after {
		rest = append(rest, values[splat:len(values)-after]...)
	}
	env.Set(names[splat], object.NewList(rest...))
	for i := 0; i < after; i++ {
		env.Set(names[splat+1+i], at(values, len(values)-after+i))
	}
}

func at(values []object.Value, i int) object.Value {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return object.Nil
}

func (in *Interp) evalCase(n *syntax.Case, env *Env, fr *Frame) (object.Value, error) {
	var subject object.Value
	if n.Subject != nil {
		v, err := in.eval(n.Subject, env, fr)
		if err != nil {
			return nil, err
		}
		subject = v
	}
	for _, w := range n.Whens {
		conds, err := in.evalArgs(w.Conds, env, fr)
		if err != nil {
			return nil, err
		}
		for _, c := range conds {
			var hit bool
			if subject == nil {
				hit = object.Truthy(c)
			} else {
				hit, err = in.caseEqual(c, subject)
				if err != nil {
					return nil, withLine(err, n.Line())
				}
			}
			if hit {
				return in.evalBody(w.Body, env, fr)
			}
		}
	}
	return in.evalBody(n.Else, env, fr)
}

// caseEqual is `pattern === value`: class membership for a class, inclusion
// for a range, a call for a proc, equality otherwise.
func (in *Interp) caseEqual(pattern, v object.Value) (bool, error) {
	switch p := pattern.(type) {
	case *object.Class:
		return v.Class().IsA(p), nil
	case *object.Range:
		return in.rangeIncludes(p, v)
	case *Proc:
		r, err := in.callBlock(p, []object.Value{v}, nil, nil)
		if err != nil {
			return false, err
		}
		return object.Truthy(r), nil
	}
	return in.Equal(pattern, v)
}

// matchRescue finds the rescue clause for an exception: its index, or -1
// when none fits.
func (in *Interp) matchRescue(rescues []syntax.Rescue, exc *object.Exception, env *Env, fr *Frame) (int, error) {
	for i := range rescues {
		r := &rescues[i]
		if len(r.Classes) == 0 {
			if exc.Cls.IsA(object.StandardError) {
				return i, nil
			}
			continue
		}
		classes, err := in.evalArgs(r.Classes, env, fr)
		if err != nil {
			return -1, err
		}
		for _, c := range classes {
			cls, ok := c.(*object.Class)
			if !ok {
				return -1, in.raise(object.TypeError, 0, "class or module required for rescue clause")
			}
			if exc.Cls.IsA(cls) {
				return i, nil
			}
		}
	}
	return -1, nil
}
