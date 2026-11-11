package eval

import (
	"math/big"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
	"github.com/cameo-lang/cameo/src/syntax"
)

// evalBody evaluates statements in order and returns the last value, nil
// when there are none.
func (in *Interp) evalBody(body []syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	var last object.Value = object.Nil
	for _, n := range body {
		if in.dbg != nil {
			if err := in.dbg.line(n.Line(), env, fr); err != nil {
				return object.Value{}, err
			}
		}
		v, err := in.eval(n, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		last = v
	}
	return last, nil
}

// eval evaluates one node: the tree walker.
func (in *Interp) eval(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	switch n := n.(type) {
	case *syntax.Nil:
		return object.Nil, nil
	case *syntax.Bool:
		return object.Bool(n.Value), nil
	case *syntax.Int:
		return object.Int(n.Value), nil
	case *syntax.BigInt:
		return in.code.literal(n, func() (object.Value, error) {
			b, _ := new(big.Int).SetString(n.Digits, 10)
			return object.BigInt(b), nil
		})
	case *syntax.Float:
		return object.Float(n.Value), nil
	case *syntax.Str:
		return object.String(n.Value), nil
	case *syntax.Sym:
		return object.Symbol(n.Name), nil
	case *syntax.Self:
		return fr.Self, nil
	case *syntax.Var:
		if v, ok := env.Get(n.Name); ok {
			return v, nil
		}
		return object.Nil, nil
	case *syntax.Assign:
		v, err := in.eval(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		env.Set(n.Name, v)
		if n.Forward {
			// The lambda reads its own name, which was unbound when
			// it captured its scope: fill it in now.
			if p, ok := asProc(v); ok && p.Env != nil {
				p.Env.bind(n.Name, v)
			}
		}
		return v, nil
	case *syntax.FieldAssign:
		v, err := in.eval(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return v, in.writeField(n.Name, v, fr, n.Line())
	case *syntax.Regexp:
		return in.evalRegexp(n, env, fr)
	case *syntax.Const:
		return in.lookupConst(n.Name, fr, n.Line())
	case *syntax.Interp:
		return in.evalInterp(n, env, fr)
	case *syntax.Call:
		return in.evalCall(n, env, fr)
	case *syntax.Lambda:
		return procValue(&Proc{Block: n.Block, Env: env.snapshot(), Frame: fr, Strict: n.Strict}), nil
	case *syntax.Return:
		v, err := in.evalOptional(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, &control{kind: ctrlReturn, value: v, target: fr, line: n.Line()}
	case *syntax.Break:
		v, err := in.evalOptional(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, &control{kind: ctrlBreak, value: v, line: n.Line()}
	case *syntax.Next:
		v, err := in.evalOptional(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, &control{kind: ctrlNext, value: v, line: n.Line()}
	case *syntax.Redo:
		return object.Value{}, &control{kind: ctrlRedo, line: n.Line()}
	case *syntax.If:
		c, err := in.eval(n.Cond, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		if c.Truthy() {
			return in.evalBody(n.Then, env, fr)
		}
		return in.evalBody(n.Else, env, fr)
	case *syntax.While:
		return in.evalWhile(n, env, fr)
	case *syntax.And:
		l, err := in.eval(n.Left, env, fr)
		if err != nil || !l.Truthy() {
			return l, err
		}
		return in.eval(n.Right, env, fr)
	case *syntax.Or:
		l, err := in.eval(n.Left, env, fr)
		if err != nil || l.Truthy() {
			return l, err
		}
		return in.eval(n.Right, env, fr)
	case *syntax.Not:
		v, err := in.eval(n.Value, env, fr)
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(!v.Truthy()), nil
	case *syntax.Begin:
		return in.evalBegin(n, env, fr)
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
		return in.evalCall(&syntax.Call{Pos: n.Pos, Name: n.Name, VCall: true}, env, fr)
	case *syntax.Seq:
		return in.evalBody(n.Body, env, fr)
	case *syntax.Alias:
		return in.evalAlias(n, fr)
	case *syntax.FieldDecl:
		return in.evalFieldDecl(n, env, fr)
	case *syntax.AttrDecl:
		return in.evalAttrDecl(n, env, fr)
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
		return in.evalAwaitAll(n, env, fr)
	case *syntax.Splat:
		return object.Value{}, in.raise(object.CameoError, n.Line(), "a splat belongs in a list or an argument list")
	}
	return object.Value{}, in.raise(object.CameoError, n.Line(), "cannot evaluate %T", n)
}

// evalOptional evaluates a node that may be absent, reading nil for it.
func (in *Interp) evalOptional(n syntax.Node, env *Env, fr *Frame) (object.Value, error) {
	if n == nil {
		return object.Nil, nil
	}
	return in.eval(n, env, fr)
}

// evalInterp evaluates an interpolated string: its parts, each converted
// with to_s, concatenated.
func (in *Interp) evalInterp(n *syntax.Interp, env *Env, fr *Frame) (object.Value, error) {
	return in.interpolate(len(n.Parts), n.Line(), func(i int) (object.Value, error) {
		return in.eval(n.Parts[i], env, fr)
	})
}

// interpolate concatenates the n parts of an interpolated string: part
// evaluates each, which is converted with to_s before the next one is
// evaluated.
func (in *Interp) interpolate(n, line int, part func(i int) (object.Value, error)) (object.Value, error) {
	var b strings.Builder
	// The result takes the encoding its parts are compatible in, as
	// Ruby's does: a binary part in UTF-8 text makes it binary.
	// UTF-8 parts alone, the usual case, need no looking at.
	kind := object.KindString
	for i := 0; i < n; i++ {
		v, err := part(i)
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

// evalRegexp evaluates a regexp literal. One without interpolation is
// made once and kept; one with it is made on every evaluation, where a
// regexp interpolated embeds as its to_s, (?flags:source), as in Ruby.
func (in *Interp) evalRegexp(n *syntax.Regexp, env *Env, fr *Frame) (object.Value, error) {
	build := func(source string) (object.Value, error) {
		re, err := object.NewRegexp(source, n.Options)
		if err != nil {
			return object.Value{}, withLine(stdlib.RegexpFailure(in, err, source), n.Line())
		}
		return object.RegexpValue(re), nil
	}
	if len(n.Parts) == 1 {
		if s, ok := n.Parts[0].(*syntax.Str); ok {
			return in.code.literal(n, func() (object.Value, error) { return build(s.Value) })
		}
	}
	var b strings.Builder
	for _, p := range n.Parts {
		v, err := in.eval(p, env, fr)
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
	return build(b.String())
}

// evalWhile evaluates while, until and their begin … end forms.
func (in *Interp) evalWhile(n *syntax.While, env *Env, fr *Frame) (object.Value, error) {
	for first := true; ; first = false {
		if err := in.stopped(); err != nil {
			return object.Value{}, err
		}
		if !n.DoWhile || !first {
			c, err := in.eval(n.Cond, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			if !c.Truthy() {
				return object.Nil, nil
			}
		}
	pass:
		if _, err := in.evalBody(n.Body, env, fr); err != nil {
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

// evalBegin evaluates begin/rescue/else/ensure.
func (in *Interp) evalBegin(n *syntax.Begin, env *Env, fr *Frame) (object.Value, error) {
	result, err := in.evalRescued(n, env, fr)
	if n.Ensure != nil {
		if _, ensureErr := in.evalBody(n.Ensure, env, fr); ensureErr != nil {
			return object.Value{}, ensureErr
		}
	}
	return result, err
}

// evalRescued evaluates the body of a begin with its rescue and else
// clauses, again from the start after a retry.
func (in *Interp) evalRescued(n *syntax.Begin, env *Env, fr *Frame) (object.Value, error) {
	for {
		result, err := in.evalBody(n.Body, env, fr)
		if err == nil {
			if n.Else != nil {
				return in.evalBody(n.Else, env, fr)
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
		result, err = in.evalBody(n.Rescues[i].Body, env, fr)
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

// evalArgs evaluates a list of expressions, splicing splats.
func (in *Interp) evalArgs(nodes []syntax.Node, env *Env, fr *Frame) ([]object.Value, error) {
	out := make([]object.Value, 0, len(nodes))
	for _, n := range nodes {
		if s, ok := n.(*syntax.Splat); ok {
			v, err := in.eval(s.Value, env, fr)
			if err != nil {
				return nil, err
			}
			if out, err = in.splat(out, v, s.Line()); err != nil {
				return nil, err
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

// splat appends what `*v` stands for to out: the elements of a list, a
// set, a range or an enumerator, nothing for nil, and any other value
// itself.
func (in *Interp) splat(out []object.Value, v object.Value, line int) ([]object.Value, error) {
	switch v.Kind() {
	case object.KindList:
		return append(out, v.List().Elems...), nil
	case object.KindSet:
		return append(out, v.Set().Elems()...), nil
	case object.KindRange:
		list, err := in.rangeToList(v.Range(), line)
		if err != nil {
			return nil, err
		}
		return append(out, list.Elems...), nil
	case object.KindEnumerator:
		err := v.Enumerator().Each(in, func(el object.Value) error {
			out = append(out, el)
			return nil
		})
		return out, err
	}
	if v.IsNil() {
		return out, nil
	}
	return append(out, v), nil
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
