package eval

import (
	"github.com/radiospiel/cameo/src/object"
	"github.com/radiospiel/cameo/src/syntax"
)

// evalBody evaluates statements in order and returns the last value, nil
// when there are none.
func (in *Interp) evalBody(body []syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	var last object.Value = object.Nil
	for _, n := range body {
		v, err := in.eval(n, env, fr)
		if err != nil {
			return object.Value{}, err
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
	case *syntax.Defined:
		return in.defined(n.Expr, env, fr)
	case *syntax.List:
		elems, err := in.evalArgs(n.Elems, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(elems...), nil
	case *syntax.Map:
		m, err := in.evalMap(n.Entries, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		return object.MapValue(m), nil
	case *syntax.Range:
		r := &object.Range{Exclusive: n.Exclusive}
		var err error
		if n.Lo != nil {
			if r.Lo, err = in.eval(n.Lo, env, fr); err != nil {
				return object.Value{}, err
			}
		}
		if n.Hi != nil {
			if r.Hi, err = in.eval(n.Hi, env, fr); err != nil {
				return object.Value{}, err
			}
		}
		return object.RangeValue(r), nil
	case *syntax.MultiAssign:
		return in.evalMultiAssign(n, env, fr)
	case *syntax.ScopedConst:
		return in.evalScopedConst(n, env, fr)
	case *syntax.ConstAssign:
		v, err := in.eval(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return v, in.defineConst(n.Name, v, fr, n.Line())
	case *syntax.Yield:
		if fr.Block.IsZero() {
			return object.Value{}, in.raise(object.LocalJumpError, n.Line(), "no block given (yield)")
		}
		args, kw, err := in.evalCallArgs(n.Args, n.KwArgs, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		v, err := in.callBlock(fr.Block, args, kw, object.Value{})
		return v, withLine(err, n.Line())
	case *syntax.Super:
		return in.evalSuper(n, env, fr)
	case *syntax.Retry:
		return object.Value{}, &control{kind: ctrlRetry, line: n.Line()}
	case *syntax.Case:
		return in.evalCase(n, env, fr)
	case *syntax.CaseIn:
		return in.evalCaseIn(n, env, fr)
	case *syntax.MatchPattern:
		return in.evalMatchPattern(n, env, fr)
	case *syntax.Def:
		return in.evalDef(n, fr)
	case *syntax.ClassDef:
		fr, err := in.scopedFrame(n.Scope, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		return in.evalClassDef(n, env, fr)
	case *syntax.ModuleDef:
		fr, err := in.scopedFrame(n.Scope, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		return in.evalModuleDef(n, env, fr)
	case *syntax.VarOrCall:
		if v, ok := env.Get(n.Name); ok {
			return v, nil
		}
		return in.evalCall(&syntax.Call{Pos: n.Pos, Name: n.Name}, env, fr)
	case *syntax.MutCall:
		return in.evalMutCall(n, env, fr)
	case *syntax.Seq:
		return in.evalBody(n.Body, env, fr)
	case *syntax.Alias:
		return in.evalAlias(n, fr)
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
		return object.Value{}, in.raise(object.CameoError, n.Line(), "a splat belongs in a list or an argument list")
	}
	return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot evaluate %T", n)
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
			switch v.Kind() {
			case object.KindList:
				v := v.List()
				out = append(out, v.Elems...)
			case object.KindSet:
				v := v.Set()
				out = append(out, v.Elems()...)
			case object.KindRange:
				v := v.Range()
				list, err := in.rangeToList(v, s.Line())
				if err != nil {
					return nil, err
				}
				out = append(out, list.Elems...)
			default:
				if v.IsNil() {
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
			if v.IsNil() {
				continue
			}
			m, ok := v.AsMap()
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
		return object.Value{}, err
	}
	var result object.Value
	if len(n.Values) == 1 {
		result = values[0]
		if l, ok := values[0].AsList(); ok {
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
			return object.Value{}, err
		}
		subject = v
	}
	for _, w := range n.Whens {
		conds, err := in.evalArgs(w.Conds, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		for _, c := range conds {
			var hit bool
			if subject.IsZero() {
				hit = c.Truthy()
			} else {
				hit, err = in.caseEqual(c, subject)
				if err != nil {
					return object.Value{}, withLine(err, n.Line())
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
	switch pattern.Kind() {
	case object.KindClass:
		p := pattern.Cls()
		return v.Class().IsA(p), nil
	case object.KindRange:
		p := pattern.Range()
		return in.rangeIncludes(p, v)
	case object.KindRegexp:
		switch v.Kind() {
		case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
			return pattern.Regexp().MatchAt(v.Str(), 0) != nil, nil
		}
		return false, nil
	case object.KindProc:
		p := proc(pattern)
		r, err := in.callBlock(procValue(p), []object.Value{v}, nil, object.Value{})
		if err != nil {
			return false, err
		}
		return r.Truthy(), nil
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
			cls, ok := c.AsClass()
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
