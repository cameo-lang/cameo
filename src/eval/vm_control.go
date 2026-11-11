package eval

import (
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// value runs a list that leaves one value and pops it.
func (m *machine) value(code []*cameopb.Code) (object.Value, error) {
	if err := m.list(code); err != nil {
		return object.Value{}, err
	}
	return m.pop(), nil
}

// regexp runs a REGEXP: a regexp literal. One without interpolation is
// made once per program.
func (m *machine) regexp(c *cameopb.Code) (object.Value, error) {
	in, options, line := m.in, int(c.Args[0].GetInt()), int(c.Line)
	part := func(i int) (object.Value, error) { return m.value(c.Code[i].GetCode()) }
	if has(c, cameopb.Flag_FLAG_STATIC) {
		return in.code.literal(c, func() (object.Value, error) {
			source, err := part(0)
			if err != nil {
				return object.Value{}, err
			}
			return in.newRegexp(source.Str(), options, line)
		})
	}
	return in.interpolateRegexp(len(c.Code), options, line, part)
}

// begin runs a BEGIN: begin/rescue/else/ensure. Its lists are the body,
// the else and the ensure, then the classes and the body of each rescue
// clause.
func (m *machine) begin(c *cameopb.Code) error {
	base := len(m.stack)
	err := m.rescued(c, base)
	if !has(c, cameopb.Flag_FLAG_ENSURE) {
		return err
	}
	if err != nil {
		m.stack = m.stack[:base]
	}
	if ensureErr := m.body(c.Code[2].GetCode()); ensureErr != nil {
		return ensureErr
	}
	m.pop()
	return err
}

// rescued runs the body of a BEGIN with its rescue and else clauses, again
// from the start after a retry. base is the height of the stack the BEGIN
// started at, which an exception out of the middle of an expression
// leaves it above.
func (m *machine) rescued(c *cameopb.Code, base int) error {
	in := m.in
	for {
		err := m.body(c.Code[0].GetCode())
		if err == nil {
			if has(c, cameopb.Flag_FLAG_ELSE) {
				m.pop()
				return m.body(c.Code[1].GetCode())
			}
			return nil
		}
		exc, ok := err.(*object.Exception)
		if !ok {
			return err
		}
		m.stack = m.stack[:base]
		i, err := m.matchRescue(c, exc)
		if err != nil {
			return err
		}
		if i < 0 {
			return exc
		}
		if in.rescuing != nil {
			// Raised inside another rescue clause and rescued before it
			// ends: it takes its cause where it is caught.
			exc = caused(exc, in.rescuing).(*object.Exception)
		}
		if v := m.name(c, i); v != "" {
			m.env.Set(v, object.ExceptionValue(exc))
		}
		outer := in.rescuing
		in.rescuing = exc
		err = m.body(c.Code[4+2*i].GetCode())
		in.rescuing = outer
		if err == nil {
			return nil
		}
		if ctl, ok := err.(*control); ok && ctl.kind == ctrlRetry {
			m.stack = m.stack[:base]
			continue
		}
		return caused(err, exc)
	}
}

// matchRescue finds the rescue clause of a BEGIN for an exception: its
// index, or -1 when none fits. A clause's classes are evaluated when an
// exception reaches it.
func (m *machine) matchRescue(c *cameopb.Code, exc *object.Exception) (int, error) {
	for i := 0; 3+2*i < len(c.Code); i++ {
		classes := c.Code[3+2*i].GetCode()
		if len(classes) == 0 {
			if exc.Cls.IsA(object.StandardError) {
				return i, nil
			}
			continue
		}
		v, err := m.value(classes)
		if err != nil {
			return -1, err
		}
		for _, cv := range v.List().Elems {
			cls, ok := cv.AsClass()
			if !ok {
				return -1, m.in.raise(object.TypeError, 0, "class or module required for rescue clause")
			}
			if exc.Cls.IsA(cls) {
				return i, nil
			}
		}
	}
	return -1, nil
}

// caseWhen runs a CASE: two lists per when, its conditions and its body,
// then the else.
func (m *machine) caseWhen(c *cameopb.Code) error {
	var subject object.Value
	if has(c, cameopb.Flag_FLAG_SUBJECT) {
		subject = m.pop()
	}
	last := len(c.Code) - 1
	for i := 0; i < last; i += 2 {
		conds, err := m.value(c.Code[i].GetCode())
		if err != nil {
			return err
		}
		for _, cond := range conds.List().Elems {
			hit := cond.Truthy()
			if !subject.IsZero() {
				if hit, err = m.in.caseEqual(cond, subject); err != nil {
					return withLine(err, int(c.Line))
				}
			}
			if hit {
				return m.body(c.Code[i+1].GetCode())
			}
		}
	}
	return m.body(c.Code[last].GetCode())
}

// caseIn runs a CASE_IN: per clause a pattern, the kind of its guard and
// two lists, the guard and the body; then the else, when it has one.
func (m *machine) caseIn(c *cameopb.Code) error {
	in, line := m.in, int(c.Line)
	subject := m.pop()
	for i := 0; 2*i < len(c.Args); i++ {
		ok, err := m.match(c.Args[2*i].GetPattern(), subject, line)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if guard := c.Args[2*i+1].GetInt(); guard != 0 {
			g, err := m.value(c.Code[2*i].GetCode())
			if err != nil {
				return err
			}
			if g.Truthy() == (guard == 2) {
				continue
			}
		}
		return m.body(c.Code[2*i+1].GetCode())
	}
	if has(c, cameopb.Flag_FLAG_ELSE) {
		return m.body(c.Code[len(c.Code)-1].GetCode())
	}
	s, _ := in.Inspect(subject)
	return in.raise(object.NoMatchingPattern, line, "%s", s)
}

// match tests a pattern against a value, binding locals on the way. A
// failed match may leave partial bindings, as in Ruby.
func (m *machine) match(p *cameopb.Pattern, v object.Value, line int) (bool, error) {
	in, env := m.in, m.env
	name := func(i uint32) string { return m.unit.Names[i] }
	switch k := p.GetKind().(type) {
	case *cameopb.Pattern_Value:
		pattern, err := m.value(k.Value.GetCode())
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
		cls, err := m.value(k.Const.GetCode())
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
	cls, err := m.value(c.GetCode())
	if err != nil {
		return false, err
	}
	return m.in.caseEqual(cls, v)
}
