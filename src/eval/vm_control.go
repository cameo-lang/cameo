package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// define runs a CLASS or a MODULE: it binds the class or module to its
// name and runs its body, a proc of the unit, in a scope and frame of its
// own.
func (m *machine) define(c *cameopb.Code) error {
	in, fr, line := m.in, m.fr, int(c.Line)
	var super object.Value
	hasSuper := has(c, cameopb.Flag_FLAG_SUPER)
	if hasSuper {
		super = m.pop()
	}
	if has(c, cameopb.Flag_FLAG_SCOPED) {
		scoped, err := in.frameIn(m.pop(), fr, line)
		if err != nil {
			return err
		}
		fr = scoped
	}
	decl := classDecl{Name: m.name(c, 0), Digest: m.name(c, 2), line: line}
	p := procOf(m.unit, c.Args[1].GetProc())
	body := func(env *Env, fr *Frame) (object.Value, error) { return in.runBody(p, env, fr) }
	var v object.Value
	var err error
	if c.Op == cameopb.Op_CLASS {
		v, err = in.defineClass(decl, super, hasSuper, body, fr)
	} else {
		v, err = in.defineModule(decl, body, fr)
	}
	if err != nil {
		return err
	}
	m.push(v)
	return nil
}

// field runs a FIELD: a typed field declaration of a class body.
func (m *machine) field(c *cameopb.Code) error {
	in, fr := m.in, m.fr
	src := c.Args[0].GetField()
	n := &fieldDecl{Type: src.GetType(), Names: src.GetNames(), Number: src.GetNumber(), Of: src.GetOf(), Key: src.GetKey(), Value: src.GetValue(), Values: src.GetValues(), line: int(c.Line)}
	var def, ref object.Value
	if has(c, cameopb.Flag_FLAG_DEFAULT) {
		def = m.pop()
	}
	if has(c, cameopb.Flag_FLAG_REF) {
		ref = m.pop()
	}
	if err := in.fieldTarget(n, fr); err != nil {
		return err
	}
	typ, err := in.fieldType(n, func() (object.Value, error) { return ref, nil }, fr)
	if err != nil {
		return err
	}
	v, err := in.declareFields(n, typ, def, fr)
	if err != nil {
		return err
	}
	m.push(v)
	return nil
}

// match tests a pattern against a value, binding locals on the way. A
// failed match may leave partial bindings, as in Ruby.
func (m *machine) match(p *cameopb.Pattern, v object.Value, line int) (bool, error) {
	in, env := m.in, m.env
	name := func(i uint32) string { return m.unit.Names[i] }
	switch k := p.GetKind().(type) {
	case *cameopb.Pattern_Value:
		pattern, err := m.nested(k.Value.GetCode())
		if err != nil {
			return false, err
		}
		return in.caseEqual(pattern, v)
	case *cameopb.Pattern_Bind:
		if n := name(k.Bind.GetName()); n != "_" {
			env.Set(n, v)
		}
		return true, nil
	case *cameopb.Pattern_Const:
		cls, err := m.nested(k.Const.GetCode())
		if err != nil {
			return false, err
		}
		return in.caseEqual(cls, v)
	case *cameopb.Pattern_Binding:
		ok, err := m.match(k.Binding.GetSub(), v, line)
		if ok {
			env.Set(name(k.Binding.GetName()), v)
		}
		return ok, err
	case *cameopb.Pattern_Alt:
		for _, alt := range k.Alt.GetAlts() {
			ok, err := m.match(alt, v, line)
			if ok || err != nil {
				return ok, err
			}
		}
		return false, nil
	case *cameopb.Pattern_List:
		p := k.List
		if ok, err := m.matchConst(p.GetConst(), v); !ok || err != nil {
			return false, err
		}
		elems, ok := in.deconstruct(v)
		if !ok {
			return false, nil
		}
		pre, post := p.GetPre(), p.GetPost()
		if p.GetHasSplat() {
			if len(elems) < len(pre)+len(post) {
				return false, nil
			}
		} else if len(elems) != len(pre) {
			return false, nil
		}
		for i, sub := range pre {
			if ok, err := m.match(sub, elems[i], line); !ok || err != nil {
				return false, err
			}
		}
		if p.GetHasSplat() {
			rest := elems[len(pre) : len(elems)-len(post)]
			if p.SplatName != nil {
				env.Set(name(p.GetSplatName()), object.NewList(append([]object.Value{}, rest...)...))
			}
			for i, sub := range post {
				if ok, err := m.match(sub, elems[len(elems)-len(post)+i], line); !ok || err != nil {
					return false, err
				}
			}
		}
		return true, nil
	case *cameopb.Pattern_Find:
		p := k.Find
		if ok, err := m.matchConst(p.GetConst(), v); !ok || err != nil {
			return false, err
		}
		elems, ok := in.deconstruct(v)
		if !ok {
			return false, nil
		}
		mid := p.GetMid()
		for start := 0; start+len(mid) <= len(elems); start++ {
			hit := true
			for i, sub := range mid {
				ok, err := m.match(sub, elems[start+i], line)
				if err != nil {
					return false, err
				}
				if !ok {
					hit = false
					break
				}
			}
			if hit {
				if p.PreName != nil {
					env.Set(name(p.GetPreName()), object.NewList(append([]object.Value{}, elems[:start]...)...))
				}
				if p.PostName != nil {
					env.Set(name(p.GetPostName()), object.NewList(append([]object.Value{}, elems[start+len(mid):]...)...))
				}
				return true, nil
			}
		}
		return false, nil
	case *cameopb.Pattern_Map:
		p := k.Map
		if ok, err := m.matchConst(p.GetConst(), v); !ok || err != nil {
			return false, err
		}
		mp, ok := in.deconstructKeys(v)
		if !ok {
			return false, nil
		}
		used := map[string]bool{}
		for _, entry := range p.GetEntries() {
			key := name(entry.GetKey())
			val, found := mp.Get(object.Symbol(key))
			if !found {
				return false, nil
			}
			used[key] = true
			if entry.GetValue() == nil {
				env.Set(key, val)
				continue
			}
			if ok, err := m.match(entry.GetValue(), val, line); !ok || err != nil {
				return false, err
			}
		}
		if p.GetRestNil() && mp.Len() != len(p.GetEntries()) {
			return false, nil
		}
		if p.GetHasRest() && p.RestName != nil {
			rest := object.NewMapBuilder()
			for i, key := range mp.Keys() {
				if s, isSym := key.AsSymbol(); isSym && used[string(s)] {
					continue
				}
				rest.Set(key, mp.Values()[i])
			}
			env.Set(name(p.GetRestName()), object.MapValue(rest.Map()))
		}
		return true, nil
	}
	return false, in.raise(object.CameoError, line, "unsupported pattern %T", p.GetKind())
}

// matchConst tests the constant of `Const[…]` or `Const(…)`, which a
// pattern without one passes.
func (m *machine) matchConst(c *cameopb.Seq, v object.Value) (bool, error) {
	if c == nil {
		return true, nil
	}
	cls, err := m.nested(c.GetCode())
	if err != nil {
		return false, err
	}
	return m.in.caseEqual(cls, v)
}
