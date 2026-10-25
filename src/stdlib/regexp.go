package stdlib

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/rx"
)

// installRegexp defines Regexp and MatchData. There are no $~ and $1
// globals, since cameo has no globals: a script reads a match from what
// match returns, or uses a block's argument.
func installRegexp() {
	c := object.RegexpClass
	c.Consts["IGNORECASE"] = object.Int(object.RegexpIgnoreCase)
	c.Consts["EXTENDED"] = object.Int(object.RegexpExtended)
	c.Consts["MULTILINE"] = object.Int(object.RegexpMultiline)
	c.Consts["FIXEDENCODING"] = object.Int(object.RegexpFixedEncoding)
	c.Consts["NOENCODING"] = object.Int(object.RegexpNoEncoding)
	re := func(self object.Value) *object.Regexp { return self.Regexp() }

	newRegexp := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		if r, ok := args[0].AsRegexp(); ok {
			return object.RegexpValue(r), nil
		}
		source, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		options := 0
		if len(args) == 2 {
			switch o := args[1]; o.Kind() {
			case object.KindInt:
				options = int(o.Int())
			case object.KindString, object.KindBytes, object.KindASCII:
				for _, f := range o.Str() {
					switch f {
					case 'i':
						options |= object.RegexpIgnoreCase
					case 'x':
						options |= object.RegexpExtended
					case 'm':
						options |= object.RegexpMultiline
					default:
						return object.Value{}, ctx.Raise(object.ArgumentError, "unknown regexp option: %s", o.Str())
					}
				}
			default:
				if o.Truthy() {
					options = object.RegexpIgnoreCase
				}
			}
		}
		return compileRegexp(ctx, source, options)
	}
	c.DefineSingleton("new", -1, newRegexp)
	c.DefineSingleton("compile", -1, newRegexp)
	escape := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(regexpEscape(s)), nil
	}
	c.DefineSingleton("escape", 1, escape)
	c.DefineSingleton("quote", 1, escape)
	c.DefineSingleton("try_convert", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v := args[0]
		if _, ok := v.AsRegexp(); ok {
			return v, nil
		}
		if !ctx.RespondsTo(v, "to_regexp") {
			return object.Nil, nil
		}
		r, err := ctx.Call(v, "to_regexp")
		if err != nil || r.IsNil() {
			return r, err
		}
		if _, ok := r.AsRegexp(); !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Regexp (%s#to_regexp gives %s)", kindName(v), kindName(v), kindName(r))
		}
		return r, nil
	})
	c.DefineSingleton("union", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 1 {
			if l, ok := args[0].AsList(); ok {
				args = l.Elems
			}
		}
		parts := make([]string, len(args))
		options := 0
		for i, a := range args {
			if r, ok := a.AsRegexp(); ok {
				parts[i] = r.String()
				continue
			}
			// Bytes make a pattern over bytes, as binary strings do.
			if b, ok := a.AsBytes(); ok {
				parts[i] = regexpEscape(string(b))
				options |= object.RegexpNoEncoding
				continue
			}
			s, err := wantString(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			parts[i] = regexpEscape(s)
		}
		if len(parts) == 0 {
			return compileRegexp(ctx, "(?!)", 0)
		}
		return compileRegexp(ctx, strings.Join(parts, "|"), options)
	})

	def(c, "source", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(re(self).Source), nil
	})
	def(c, "options", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(re(self).Options), nil
	})
	def(c, "fixed_encoding?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(re(self).Options&object.RegexpFixedEncoding != 0), nil
	})
	def(c, "casefold?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(re(self).Options&object.RegexpIgnoreCase != 0), nil
	})
	def(c, "names", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return stringList(re(self).UniqueNames()), nil
	})
	def(c, "named_captures", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for _, name := range re(self).UniqueNames() {
			idx := re(self).GroupIndexes(name)
			out := make([]object.Value, len(idx))
			for k, i := range idx {
				out[k] = object.Int(i)
			}
			b.Set(object.String(name), object.NewList(out...))
		}
		return b.Value(), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(re(self).String()), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(re(self).Inspect()), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
	def(c, "eql?", 1, c.Methods["=="].Native)
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "match", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		if args[0].IsNil() {
			return object.Nil, nil
		}
		return matchFrom(ctx, re(self), args, block)
	})
	def(c, "match?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		if args[0].IsNil() {
			return object.False, nil
		}
		m, err := matchFrom(ctx, re(self), args, object.Value{})
		return object.Bool(err == nil && !m.IsNil()), err
	})
	def(c, "=~", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsNil() {
			return object.Nil, nil
		}
		s, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return matchIndex(re(self), s), nil
	})
	def(c, "===", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var s string
		switch args[0].Kind() {
		case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
			s = args[0].Str()
		default:
			if !ctx.RespondsTo(args[0], "to_str") {
				return object.False, nil
			}
			v, err := ctx.Call(args[0], "to_str")
			if err != nil {
				return object.Value{}, err
			}
			if s, err = wantString(ctx, v); err != nil {
				return object.Value{}, err
			}
		}
		return object.Bool(re(self).MatchAt(s, 0) != nil), nil
	})

	installMatchData()
}

func compileRegexp(ctx object.Context, source string, options int) (object.Value, error) {
	r, err := object.NewRegexp(source, options)
	if err != nil {
		return object.Value{}, RegexpFailure(ctx, err, source)
	}
	return object.RegexpValue(r), nil
}

// RegexpFailure is the error for a pattern that does not compile: a
// NotImplementedError for one Go's regexp cannot match, which says so, and
// a RegexpError for a malformed one.
func RegexpFailure(ctx object.Context, err error, source string) error {
	var unsupported *rx.Unsupported
	if errors.As(err, &unsupported) {
		return ctx.Raise(object.NotImplementedErr, "%s", unsupported.Error())
	}
	return ctx.Raise(object.RegexpError, "%s: /%s/", err.Error(), source)
}

func stringList(ss []string) object.Value {
	out := make([]object.Value, len(ss))
	for i, s := range ss {
		out[i] = object.String(s)
	}
	return object.NewList(out...)
}

// regexpEscape is Regexp.escape: the characters special in a pattern,
// backslashed, and whitespace as escapes.
func regexpEscape(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch r {
		case '.', '*', '?', '+', '^', '$', '|', '(', ')', '[', ']', '{', '}', '\\', '/', '-', '#':
			b.WriteByte('\\')
			b.WriteRune(r)
		case ' ':
			b.WriteString(`\ `)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		default:
			// A byte that is not UTF-8 stays the byte it is.
			_, size := utf8.DecodeRuneInString(s[i:])
			b.WriteString(s[i : i+size])
		}
	}
	return b.String()
}

// matchFrom is Regexp#match(str, pos = 0): a MatchData or nil, yielded to
// a block when there is one.
func matchFrom(ctx object.Context, r *object.Regexp, args []object.Value, block object.Value) (object.Value, error) {
	var s string
	switch args[0].Kind() {
	case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
		s = args[0].Str()
	default:
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(args[0]))
	}
	pos := 0
	if len(args) == 2 {
		p, err := wantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
		}
		pos = charToByte(s, int(p))
		if pos < 0 {
			return object.Nil, nil
		}
	}
	m := r.MatchAt(s, pos)
	if m == nil {
		return object.Nil, nil
	}
	if !block.IsZero() {
		return ctx.Yield(block, object.MatchValue(m))
	}
	return object.MatchValue(m), nil
}

// matchIndex is =~: the character offset of the first match, or nil.
func matchIndex(r *object.Regexp, s string) object.Value {
	m := r.MatchAt(s, 0)
	if m == nil {
		return object.Nil
	}
	return object.Int(m.CharOffset(m.Caps[0]))
}

// charToByte is the byte offset of character i of s, counting from the end
// for a negative i; -1 past either end.
func charToByte(s string, i int) int {
	n := utf8.RuneCountInString(s)
	if i < 0 {
		i += n
	}
	if i < 0 || i > n {
		return -1
	}
	b := 0
	for k := 0; k < i; k++ {
		_, size := utf8.DecodeRuneInString(s[b:])
		b += size
	}
	return b
}

func installMatchData() {
	c := object.MatchDataClass
	md := func(self object.Value) *object.Match { return self.Match() }
	group := func(m *object.Match, i int) object.Value {
		if s, ok := m.Group(i); ok {
			return object.String(s)
		}
		return object.Nil
	}
	all := func(m *object.Match) *object.List {
		out := make([]object.Value, m.Groups()+1)
		for i := range out {
			out[i] = group(m, i)
		}
		return object.NewList(out...).List()
	}
	// named is the group a name or symbol refers to.
	named := func(ctx object.Context, m *object.Match, v object.Value) (int, error) {
		if i := m.NameIndex(v.Str()); i >= 0 {
			return i, nil
		}
		return 0, ctx.Raise(object.IndexError, "undefined group name reference: %s", v.Str())
	}
	isName := func(v object.Value) bool {
		switch v.Kind() {
		case object.KindString, object.KindBytes, object.KindASCII, object.KindSymbol:
			return true
		}
		return false
	}
	// at is the group a number, name or symbol refers to, as begin, end
	// and match read it: a number past either end, a negative one
	// included, raises IndexError.
	at := func(ctx object.Context, m *object.Match, v object.Value) (int, error) {
		if isName(v) {
			return named(ctx, m, v)
		}
		n, err := wantInt(ctx, v, "")
		if err != nil {
			return 0, err
		}
		if n < 0 || n > int64(m.Groups()) {
			return 0, ctx.Raise(object.IndexError, "index %d out of matches", n)
		}
		return int(n), nil
	}
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := md(self)
		if len(args) == 1 && isName(args[0]) {
			i, err := named(ctx, m, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return group(m, i), nil
		}
		if len(args) == 1 && args[0].Kind() == object.KindFloat {
			args = []object.Value{object.Int(int64(args[0].Float()))}
		}
		return listIndex(ctx, all(m), args)
	})
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ListValue(all(md(self))), nil
	})
	captures := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, md(self).Groups())
		for i := range out {
			out[i] = group(md(self), i+1)
		}
		return object.NewList(out...), nil
	}
	def(c, "captures", 0, captures)
	def(c, "deconstruct", 0, captures)
	def(c, "named_captures", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 0 {
			return object.Value{}, argError(ctx, len(args), "0")
		}
		symbols := false
		if kw != nil {
			for i, k := range kw.Keys() {
				if k.Kind() == object.KindSymbol && k.Str() == "symbolize_names" {
					symbols = kw.Values()[i].Truthy()
					continue
				}
				s, err := ctx.Inspect(k)
				if err != nil {
					return object.Value{}, err
				}
				return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
			}
		}
		m := md(self)
		b := object.NewMapBuilder()
		for _, name := range m.Re.UniqueNames() {
			k := object.String(name)
			if symbols {
				k = object.Symbol(name)
			}
			b.Set(k, group(m, m.NameIndex(name)))
		}
		return b.Value(), nil
	})
	def(c, "deconstruct_keys", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := md(self)
		names := m.Re.UniqueNames()
		b := object.NewMapBuilder()
		if args[0].IsNil() {
			for _, name := range names {
				b.Set(object.Symbol(name), group(m, m.NameIndex(name)))
			}
			return b.Value(), nil
		}
		keys, ok := args[0].AsList()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Array)", kindName(args[0]))
		}
		// More keys than names cannot all be there, so none are read.
		if keys.Len() > len(names) {
			return b.Value(), nil
		}
		for _, k := range keys.Elems {
			if k.Kind() != object.KindSymbol {
				return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Symbol)", kindName(k))
			}
			i := m.NameIndex(k.Str())
			if i < 0 {
				break
			}
			b.Set(k, group(m, i))
		}
		return b.Value(), nil
	})
	def(c, "names", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return stringList(md(self).Re.UniqueNames()), nil
	})
	def(c, "values_at", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := md(self)
		groups := all(m)
		n := int64(groups.Len())
		out := make([]object.Value, 0, len(args))
		for _, a := range args {
			switch {
			case isName(a):
				i, err := named(ctx, m, a)
				if err != nil {
					return object.Value{}, err
				}
				out = append(out, group(m, i))
			case a.Kind() == object.KindRange:
				lo, hi, err := valuesAtBounds(ctx, a.Range(), n)
				if err != nil {
					return object.Value{}, err
				}
				for i := lo; i <= hi; i++ {
					out = append(out, group(m, int(i)))
				}
			default:
				i, err := wantInt(ctx, a, "")
				if err != nil {
					return object.Value{}, err
				}
				if i < 0 {
					i += n
				}
				out = append(out, group(m, int(i)))
			}
		}
		return object.NewList(out...), nil
	})
	// span is the byte span of a group, -1 for one that took no part.
	span := func(ctx object.Context, self object.Value, v object.Value) (int, int, error) {
		m := md(self)
		i, err := at(ctx, m, v)
		if err != nil {
			return 0, 0, err
		}
		return m.Caps[2*i], m.Caps[2*i+1], nil
	}
	offset := func(m *object.Match, b int, bytes bool) object.Value {
		switch {
		case b < 0:
			return object.Nil
		case bytes:
			return object.Int(b)
		}
		return object.Int(m.CharOffset(b))
	}
	bound := func(end, bytes bool) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			b, e, err := span(ctx, self, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if end {
				b = e
			}
			return offset(md(self), b, bytes), nil
		}
	}
	def(c, "begin", 1, bound(false, false))
	def(c, "end", 1, bound(true, false))
	def(c, "bytebegin", 1, bound(false, true))
	def(c, "byteend", 1, bound(true, true))
	offsets := func(bytes bool) fn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			b, e, err := span(ctx, self, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.NewList(offset(md(self), b, bytes), offset(md(self), e, bytes)), nil
		}
	}
	def(c, "offset", 1, offsets(false))
	def(c, "byteoffset", 1, offsets(true))
	def(c, "match", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := at(ctx, md(self), args[0])
		if err != nil {
			return object.Value{}, err
		}
		return group(md(self), i), nil
	})
	def(c, "match_length", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := at(ctx, md(self), args[0])
		if err != nil {
			return object.Value{}, err
		}
		s, ok := md(self).Group(i)
		if !ok {
			return object.Nil, nil
		}
		return object.Int(utf8.RuneCountInString(s)), nil
	})
	def(c, "pre_match", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(md(self).Input[:md(self).Caps[0]]), nil
	})
	def(c, "post_match", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(md(self).Input[md(self).Caps[1]:]), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return group(md(self), 0), nil
	})
	def(c, "string", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(md(self).Input), nil
	})
	def(c, "regexp", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.RegexpValue(md(self).Re), nil
	})
	size := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(md(self).Groups() + 1), nil
	}
	def(c, "size", 0, size)
	def(c, "length", 0, size)
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(MatchInspect(md(self))), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
}

// MatchInspect is MatchData#inspect: #<MatchData "whole" 1:"group" name:"group">.
func MatchInspect(m *object.Match) string {
	var b strings.Builder
	b.WriteString("#<MatchData ")
	quote := func(i int) string {
		if s, ok := m.Group(i); ok {
			return object.QuoteString(s)
		}
		return "nil"
	}
	b.WriteString(quote(0))
	for i := 1; i <= m.Groups(); i++ {
		b.WriteByte(' ')
		if len(m.Re.Names) >= i {
			b.WriteString(m.Re.Names[i-1])
		} else {
			b.WriteString(strconv.Itoa(i))
		}
		b.WriteByte(':')
		b.WriteString(quote(i))
	}
	b.WriteByte('>')
	return b.String()
}

// A matcher finds a pattern, a string or a regexp, in a string: what
// String's sub, gsub, scan, split, index and their kin take.
type matcher struct {
	lit string
	re  *object.Regexp
}

func patternOf(ctx object.Context, v object.Value) (matcher, error) {
	if r, ok := v.AsRegexp(); ok {
		return matcher{re: r}, nil
	}
	s, err := wantString(ctx, v)
	if err != nil {
		return matcher{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Regexp)", kindName(v))
	}
	return matcher{lit: s}, nil
}

// findAt finds the first match at or after byte offset pos.
func (p matcher) findAt(s string, pos int) *object.Match {
	if p.re != nil {
		return p.re.MatchAt(s, pos)
	}
	i := strings.Index(s[pos:], p.lit)
	if i < 0 {
		return nil
	}
	return &object.Match{Input: s, Caps: []int{pos + i, pos + i + len(p.lit)}, Re: &object.Regexp{}}
}

// findHere is a match starting at byte offset pos, or nil.
func (p matcher) findHere(s string, pos int) *object.Match {
	if p.re != nil {
		return p.re.MatchHere(s, pos)
	}
	if !strings.HasPrefix(s[pos:], p.lit) {
		return nil
	}
	return &object.Match{Input: s, Caps: []int{pos, pos + len(p.lit)}, Re: &object.Regexp{}}
}

// findAll is every match, left to right; an empty match advances one
// character.
func (p matcher) findAll(s string, limit int) []*object.Match {
	if p.re != nil {
		return p.re.FindAll(s, limit)
	}
	var out []*object.Match
	for pos := 0; pos <= len(s) && (limit < 0 || len(out) < limit); {
		m := p.findAt(s, pos)
		if m == nil {
			break
		}
		out = append(out, m)
		if m.Caps[1] > m.Caps[0] {
			pos = m.Caps[1]
			continue
		}
		if m.Caps[1] >= len(s) {
			break
		}
		_, size := utf8.DecodeRuneInString(s[m.Caps[1]:])
		pos = m.Caps[1] + size
	}
	return out
}

// expand is a replacement string with its backreferences filled in: \0
// and \& the match, \1 to \9 the groups, \k<name> a named group, \` and
// \' the text before and after, and \\ a backslash.
func expand(repl string, m *object.Match) string {
	if !strings.Contains(repl, `\`) {
		return repl
	}
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c != '\\' || i+1 == len(repl) {
			b.WriteByte(c)
			continue
		}
		i++
		switch d := repl[i]; {
		case d >= '0' && d <= '9':
			s, _ := m.Group(int(d - '0'))
			b.WriteString(s)
		case d == '&':
			s, _ := m.Group(0)
			b.WriteString(s)
		case d == '`':
			b.WriteString(m.Input[:m.Caps[0]])
		case d == '\'':
			b.WriteString(m.Input[m.Caps[1]:])
		case d == '\\':
			b.WriteByte('\\')
		case d == 'k' && i+1 < len(repl) && repl[i+1] == '<':
			if end := strings.IndexByte(repl[i:], '>'); end > 0 {
				s, _ := m.Group(m.NameIndex(repl[i+2 : i+end]))
				b.WriteString(s)
				i += end
				continue
			}
			b.WriteString(`\k`)
		default:
			b.WriteByte('\\')
			b.WriteByte(d)
		}
	}
	return b.String()
}

// matchValue is what a block gets for a match: the matched text.
func matchText(m *object.Match) object.Value {
	s, _ := m.Group(0)
	return object.String(s)
}

// groupsOrText is what scan gives for a match: the groups when the pattern
// has any, the matched text otherwise.
func groupsOrText(m *object.Match) object.Value {
	if m.Groups() == 0 {
		return matchText(m)
	}
	out := make([]object.Value, m.Groups())
	for i := range out {
		if s, ok := m.Group(i + 1); ok {
			out[i] = object.String(s)
		} else {
			out[i] = object.Nil
		}
	}
	return object.NewList(out...)
}
