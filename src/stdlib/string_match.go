package stdlib

import (
	"strings"

	"github.com/radiospiel/cameo/src/object"
)

// installStringPatterns defines the String methods that take a pattern, a
// string or a regexp alike: sub, gsub, scan, split, index, rindex, [],
// slice, match, match?, =~, start_with?, partition and rpartition.
func installStringPatterns() {
	c := object.StringClass
	str := func(self object.Value) string { return self.Str() }

	def(c, "sub", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return substitute(ctx, str(self), args, kw, block, 1, self.Kind() == object.KindBytes)
	})
	def(c, "gsub", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return substitute(ctx, str(self), args, kw, block, -1, self.Kind() == object.KindBytes)
	})
	def(c, "scan", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := patternOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var out []object.Value
		for _, m := range p.findAll(str(self), -1) {
			v := groupsOrText(m)
			if block.IsZero() {
				out = append(out, v)
				continue
			}
			if _, err := ctx.Yield(block, v); err != nil {
				return object.Value{}, err
			}
		}
		if block.IsZero() {
			return object.NewList(out...), nil
		}
		return self, nil
	})
	def(c, "split", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return split(ctx, str(self), args)
	})
	def(c, "=~", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		r, ok := args[0].AsRegexp()
		if !ok {
			if _, isStr := args[0].AsString(); isStr {
				return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type String (expected Regexp)")
			}
			return ctx.Call(args[0], "=~", self)
		}
		return matchIndex(r, str(self)), nil
	})
	asRegexp := func(ctx object.Context, v object.Value) (*object.Regexp, error) {
		if r, ok := v.AsRegexp(); ok {
			return r, nil
		}
		// A string pattern is compiled as a regexp, as Ruby's match does.
		s, err := wantString(ctx, v)
		if err != nil {
			return nil, err
		}
		r, err := compileRegexp(ctx, s, 0)
		if err != nil {
			return nil, err
		}
		return r.Regexp(), nil
	}
	def(c, "match", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		r, err := asRegexp(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return matchFrom(ctx, r, append([]object.Value{self}, args[1:]...), block)
	})
	def(c, "match?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		r, err := asRegexp(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		m, err := matchFrom(ctx, r, append([]object.Value{self}, args[1:]...), object.Value{})
		return object.Bool(err == nil && !m.IsNil()), err
	})
	// index and rindex find the first match at or after a position, and the
	// last one starting at or before it.
	index := func(last bool) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, argError(ctx, len(args), "1..2")
			}
			s := str(self)
			p, err := patternOf(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			start := 0
			if last {
				start = len(s)
			}
			if len(args) == 2 {
				i, err := wantInt(ctx, args[1], "")
				if err != nil {
					return object.Value{}, err
				}
				if start = charToByte(s, int(i)); start < 0 {
					return object.Nil, nil
				}
			}
			if !last {
				if m := p.findAt(s, start); m != nil {
					return object.Int(m.CharOffset(m.Caps[0])), nil
				}
				return object.Nil, nil
			}
			for pos := start; pos >= 0; pos-- {
				if m := p.findHere(s, pos); m != nil {
					return object.Int(m.CharOffset(pos)), nil
				}
			}
			return object.Nil, nil
		}
	}
	def(c, "index", -1, index(false))
	def(c, "rindex", -1, index(true))
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) >= 1 {
			if r, ok := args[0].AsRegexp(); ok {
				m := r.MatchAt(str(self), 0)
				if m == nil {
					return object.Nil, nil
				}
				g := 0
				if len(args) == 2 {
					switch a := args[1]; a.Kind() {
					case object.KindInt:
						g = int(a.Int())
					case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
						if g = r.GroupIndex(a.Str()); g < 0 {
							return object.Value{}, ctx.Raise(object.IndexError, "undefined group name reference: %s", a.Str())
						}
					}
				}
				if s, ok := m.Group(g); ok {
					return object.String(s), nil
				}
				return object.Nil, nil
			}
		}
		return stringIndex(ctx, str(self), args)
	})
	def(c, "slice", -1, c.Methods["[]"].Native)
	def(c, "start_with?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			if r, ok := a.AsRegexp(); ok {
				if m := r.MatchHere(str(self), 0); m != nil {
					return object.True, nil
				}
				continue
			}
			p, err := wantString(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			if strings.HasPrefix(str(self), p) {
				return object.True, nil
			}
		}
		return object.False, nil
	})
	parts := func(m *object.Match) object.Value {
		return object.NewList(object.String(m.Input[:m.Caps[0]]), matchText(m), object.String(m.Input[m.Caps[1]:]))
	}
	def(c, "partition", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p, err := patternOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if m := p.findAt(str(self), 0); m != nil {
			return parts(m), nil
		}
		return object.NewList(self, object.String(""), object.String("")), nil
	})
	def(c, "rpartition", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s := str(self)
		p, err := patternOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		var last *object.Match
		if p.re == nil {
			if i := strings.LastIndex(s, p.lit); i >= 0 {
				last = &object.Match{Input: s, Caps: []int{i, i + len(p.lit)}, Re: &object.Regexp{}}
			}
		} else {
			// The last position a match starts at, searching backwards as
			// Ruby's rpartition does.
			for pos := len(s); pos >= 0 && last == nil; pos-- {
				if m := p.re.MatchHere(s, pos); m != nil {
					last = m
				}
			}
		}
		if last == nil {
			return object.NewList(object.String(""), object.String(""), self), nil
		}
		return parts(last), nil
	})
}

// substitute is sub (limit 1) and gsub (limit -1): each match replaced by
// a string with its backreferences filled in, by a hash's value for the
// matched text, or by a block's value for it.
func substitute(ctx object.Context, s string, args []object.Value, kw *object.Map, block object.Value, limit int, binary bool) (object.Value, error) {
	// `gsub(/a/, "a" => "b")` passes its hash as keywords.
	if kw != nil && kw.Len() > 0 && len(args) == 1 {
		args = append(args, object.MapValue(kw))
	}
	if len(args) < 1 || len(args) > 2 || (len(args) == 1 && block.IsZero()) {
		return object.Value{}, argError(ctx, len(args), "2")
	}
	p, err := patternOf(ctx, args[0])
	if err != nil {
		return object.Value{}, err
	}
	if len(args) == 2 {
		switch args[1].Kind() {
		case object.KindString, object.KindASCII, object.KindMap:
		case object.KindBytes:
			args = []object.Value{args[0], object.String(string(args[1].Bytes()))}
		default:
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(args[1]))
		}
	}
	// Over bytes the matched text, a hash's keys and a block's argument
	// are bytes too.
	text := func(m *object.Match) object.Value {
		if binary {
			g, _ := m.Group(0)
			return object.Bytes([]byte(g))
		}
		return matchText(m)
	}
	textOrToS := func(v object.Value) (string, error) {
		if b, ok := v.AsBytes(); ok {
			return string(b), nil
		}
		return ctx.ToS(v)
	}
	var b strings.Builder
	at := 0
	for _, m := range p.findAll(s, limit) {
		b.WriteString(s[at:m.Caps[0]])
		var repl string
		switch {
		case len(args) == 2 && args[1].IsString():
			repl = expand(args[1].Str(), m)
		case len(args) == 2:
			if v, ok := args[1].Map().Get(text(m)); ok {
				if repl, err = textOrToS(v); err != nil {
					return object.Value{}, err
				}
			}
		default:
			v, err := ctx.Yield(block, text(m))
			if err != nil {
				return object.Value{}, err
			}
			if repl, err = textOrToS(v); err != nil {
				return object.Value{}, err
			}
		}
		b.WriteString(repl)
		at = m.Caps[1]
	}
	b.WriteString(s[at:])
	if binary {
		return object.Bytes([]byte(b.String())), nil
	}
	return object.String(b.String()), nil
}

// split is String#split: on whitespace by default (and for " "), into
// characters for "", and on each match of a string or regexp otherwise,
// with a regexp's groups kept between the fields. A positive limit caps
// the fields; without one, trailing empty fields are dropped.
func split(ctx object.Context, s string, args []object.Value) (object.Value, error) {
	limit := 0
	if len(args) >= 2 {
		n, err := wantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
		}
		limit = int(n)
	}
	var fields []object.Value
	add := func(f string) { fields = append(fields, object.String(f)) }
	var p matcher
	whitespace := len(args) == 0 || args[0].IsNil()
	if !whitespace {
		var err error
		if p, err = patternOf(ctx, args[0]); err != nil {
			return object.Value{}, err
		}
		whitespace = p.re == nil && p.lit == " "
	}
	switch {
	case whitespace:
		rest := strings.TrimLeft(s, " \t\n\v\f\r")
		for rest != "" {
			if limit > 0 && len(fields) == limit-1 {
				add(rest)
				rest = ""
				break
			}
			end := strings.IndexAny(rest, " \t\n\v\f\r")
			if end < 0 {
				add(rest)
				break
			}
			add(rest[:end])
			rest = strings.TrimLeft(rest[end:], " \t\n\v\f\r")
		}
	case p.re == nil && p.lit == "":
		for _, r := range s {
			if limit > 0 && len(fields) == limit-1 {
				add(s[len(fieldsText(fields)):])
				break
			}
			add(string(r))
		}
	default:
		at := 0
		for _, m := range p.findAll(s, -1) {
			if limit > 0 && len(fields) >= limit-1 {
				break
			}
			if m.Caps[1] == m.Caps[0] {
				// An empty match splits between characters, never at the
				// start or the end.
				if m.Caps[0] == 0 || m.Caps[0] >= len(s) {
					continue
				}
			}
			add(s[at:m.Caps[0]])
			for i := 1; i <= m.Groups(); i++ {
				if g, ok := m.Group(i); ok {
					add(g)
				}
			}
			at = m.Caps[1]
		}
		add(s[at:])
	}
	if limit == 0 {
		for len(fields) > 0 && fields[len(fields)-1].Str() == "" {
			fields = fields[:len(fields)-1]
		}
	}
	return object.NewList(fields...), nil
}

func fieldsText(fields []object.Value) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f.Str())
	}
	return b.String()
}
