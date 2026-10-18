package eval

import (
	"github.com/radiospiel/cameo/src/object"
	"github.com/radiospiel/cameo/src/syntax"
)

func (in *Interp) evalCaseIn(n *syntax.CaseIn, env *Env, fr *Frame) (object.Value, error) {
	subject, err := in.eval(n.Subject, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	for _, cl := range n.Clauses {
		ok, err := in.match(cl.Pattern, subject, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		if !ok {
			continue
		}
		if cl.Guard != nil {
			g, err := in.eval(cl.Guard, env, fr)
			if err != nil {
				return object.Value{}, err
			}
			if g.Truthy() == cl.GuardNeg {
				continue
			}
		}
		return in.evalBody(cl.Body, env, fr)
	}
	if n.Else != nil {
		return in.evalBody(n.Else, env, fr)
	}
	s, _ := in.Inspect(subject)
	return object.Value{}, in.raise(object.NoMatchingPattern, n.Line(), "%s", s)
}

func (in *Interp) evalMatchPattern(n *syntax.MatchPattern, env *Env, fr *Frame) (object.Value, error) {
	subject, err := in.eval(n.Subject, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	ok, err := in.match(n.Pattern, subject, env, fr, n.Line())
	if err != nil {
		return object.Value{}, err
	}
	if n.Test {
		return object.Bool(ok), nil
	}
	if !ok {
		s, _ := in.Inspect(subject)
		return object.Value{}, in.raise(object.NoMatchingPattern, n.Line(), "%s", s)
	}
	return object.Nil, nil
}

// match tests a pattern against a value, binding locals on the way. A failed
// match may leave partial bindings, as in Ruby.
func (in *Interp) match(p syntax.Pattern, v object.Value, env *Env, fr *Frame, line int) (bool, error) {
	switch p := p.(type) {
	case *syntax.ValuePattern:
		pattern, err := in.eval(p.Value, env, fr)
		if err != nil {
			return false, err
		}
		return in.caseEqual(pattern, v)
	case *syntax.BindPattern:
		if p.Name != "_" {
			env.Set(p.Name, v)
		}
		return true, nil
	case *syntax.ConstPattern:
		c, err := in.eval(p.Const, env, fr)
		if err != nil {
			return false, err
		}
		return in.caseEqual(c, v)
	case *syntax.BindingPattern:
		ok, err := in.match(p.Sub, v, env, fr, line)
		if ok {
			env.Set(p.Name, v)
		}
		return ok, err
	case *syntax.AltPattern:
		for _, alt := range p.Alts {
			ok, err := in.match(alt, v, env, fr, line)
			if ok || err != nil {
				return ok, err
			}
		}
		return false, nil
	case *syntax.ListPattern:
		if ok, err := in.matchConst(p.Const, v, env, fr); !ok || err != nil {
			return false, err
		}
		elems, ok := in.deconstruct(v)
		if !ok {
			return false, nil
		}
		if p.HasSplat {
			if len(elems) < len(p.Pre)+len(p.Post) {
				return false, nil
			}
		} else if len(elems) != len(p.Pre) {
			return false, nil
		}
		for i, sub := range p.Pre {
			if ok, err := in.match(sub, elems[i], env, fr, line); !ok || err != nil {
				return false, err
			}
		}
		if p.HasSplat {
			rest := elems[len(p.Pre) : len(elems)-len(p.Post)]
			if p.SplatName != "" {
				env.Set(p.SplatName, object.NewList(append([]object.Value{}, rest...)...))
			}
			for i, sub := range p.Post {
				if ok, err := in.match(sub, elems[len(elems)-len(p.Post)+i], env, fr, line); !ok || err != nil {
					return false, err
				}
			}
		}
		return true, nil
	case *syntax.FindPattern:
		if ok, err := in.matchConst(p.Const, v, env, fr); !ok || err != nil {
			return false, err
		}
		elems, ok := in.deconstruct(v)
		if !ok {
			return false, nil
		}
		for start := 0; start+len(p.Mid) <= len(elems); start++ {
			hit := true
			for i, sub := range p.Mid {
				ok, err := in.match(sub, elems[start+i], env, fr, line)
				if err != nil {
					return false, err
				}
				if !ok {
					hit = false
					break
				}
			}
			if hit {
				if p.PreName != "" {
					env.Set(p.PreName, object.NewList(append([]object.Value{}, elems[:start]...)...))
				}
				if p.PostName != "" {
					env.Set(p.PostName, object.NewList(append([]object.Value{}, elems[start+len(p.Mid):]...)...))
				}
				return true, nil
			}
		}
		return false, nil
	case *syntax.MapPattern:
		if ok, err := in.matchConst(p.Const, v, env, fr); !ok || err != nil {
			return false, err
		}
		m, ok := in.deconstructKeys(v)
		if !ok {
			return false, nil
		}
		used := map[string]bool{}
		for i, key := range p.Keys {
			val, found := m.Get(object.Symbol(key))
			if !found {
				return false, nil
			}
			used[key] = true
			if p.Values[i] == nil {
				env.Set(key, val)
				continue
			}
			if ok, err := in.match(p.Values[i], val, env, fr, line); !ok || err != nil {
				return false, err
			}
		}
		if p.RestNil && m.Len() != len(p.Keys) {
			return false, nil
		}
		if p.HasRest && p.RestName != "" {
			rest := object.NewMapBuilder()
			for i, k := range m.Keys() {
				if s, isSym := k.AsSymbol(); isSym && used[string(s)] {
					continue
				}
				rest.Set(k, m.Values()[i])
			}
			env.Set(p.RestName, object.MapValue(rest.Map()))
		}
		return true, nil
	}
	return false, in.raise(object.CameoError, line, "unsupported pattern %T", p)
}

func (in *Interp) matchConst(c syntax.Node, v object.Value, env *Env, fr *Frame) (bool, error) {
	if c == nil {
		return true, nil
	}
	cls, err := in.eval(c, env, fr)
	if err != nil {
		return false, err
	}
	return in.caseEqual(cls, v)
}

// deconstruct returns a value's elements for a list pattern: a list's
// elements, an instance's fields.
func (in *Interp) deconstruct(v object.Value) ([]object.Value, bool) {
	switch v.Kind() {
	case object.KindList:
		v := v.List()
		return v.Elems, true
	case object.KindInstance:
		v := v.Instance()
		out := make([]object.Value, len(v.Fields))
		for i, f := range v.Fields {
			out[i] = object.OrNil(f)
		}
		return out, true
	}
	return nil, false
}

// deconstructKeys returns a value as a map for a map pattern: a map itself,
// an instance's fields by name.
func (in *Interp) deconstructKeys(v object.Value) (*object.Map, bool) {
	switch v.Kind() {
	case object.KindMap:
		v := v.Map()
		return v, true
	case object.KindInstance:
		v := v.Instance()
		b := object.NewMapBuilder()
		for i, f := range v.Cls.Fields {
			b.Set(object.Symbol(f.Name), object.OrNil(v.Fields[i]))
		}
		return b.Map(), true
	}
	return nil, false
}
