package stdlib

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/radiospiel/amber/src/object"
)

func installString() {
	c := object.StringClass
	str := func(self object.Value) string { return string(self.Str()) }
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsString()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(args[0]))
		}
		return object.String(str(self) + string(o)), nil
	})
	def(c, "*", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative argument")
		}
		return object.String(strings.Repeat(str(self), int(n))), nil
	})
	def(c, "%", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var fargs []object.Value
		if l, ok := args[0].AsList(); ok {
			fargs = l.Elems
		} else {
			fargs = args
		}
		s, err := format(ctx, str(self), fargs)
		return object.String(s), err
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsString()
		return object.Bool(ok && o == self.Str()), nil
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsString()
		if !ok {
			return object.Nil, nil
		}
		return object.Int(strings.Compare(str(self), string(o))), nil
	})
	def(c, "eql?", 1, c.Methods["=="].Native)
	def(c, "===", 1, c.Methods["=="].Native)
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(utf8.RuneCountInString(str(self))), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "bytesize", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(len(str(self))), nil
	})
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(str(self) == ""), nil
	})
	mapStr := func(name string, f func(string) string) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.String(f(str(self))), nil
		})
	}
	mapStr("upcase", strings.ToUpper)
	mapStr("downcase", strings.ToLower)
	mapStr("capitalize", func(s string) string {
		if s == "" {
			return s
		}
		r, size := utf8.DecodeRuneInString(s)
		return string(unicode.ToUpper(r)) + strings.ToLower(s[size:])
	})
	mapStr("swapcase", func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsUpper(r) {
				return unicode.ToLower(r)
			}
			return unicode.ToUpper(r)
		}, s)
	})
	mapStr("reverse", func(s string) string {
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	})
	mapStr("strip", func(s string) string { return strings.Trim(s, " \t\n\r\f\v\x00") })
	mapStr("lstrip", func(s string) string { return strings.TrimLeft(s, " \t\n\r\f\v\x00") })
	mapStr("rstrip", func(s string) string { return strings.TrimRight(s, " \t\n\r\f\v\x00") })
	mapStr("chomp", func(s string) string {
		s = strings.TrimSuffix(s, "\n")
		return strings.TrimSuffix(s, "\r")
	})
	mapStr("chop", func(s string) string {
		if s == "" {
			return s
		}
		_, size := utf8.DecodeLastRuneInString(s)
		return s[:len(s)-size]
	})
	mapStr("succ", succ)
	mapStr("next", succ)
	// upto yields self and its successors up to max, as a range of strings
	// from self to max does.
	def(c, "upto", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		hi, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		exclusive := len(args) == 2 && args[1].Truthy()
		out := []object.Value{}
		for s := range stringsUpto(self.Str(), hi, exclusive, false) {
			if block.IsZero() {
				out = append(out, object.String(s))
			} else if _, err := ctx.Yield(block, object.String(s)); err != nil {
				return object.Value{}, err
			}
		}
		if block.IsZero() {
			return object.NewList(out...), nil
		}
		return self, nil
	})
	mapStr("to_s", func(s string) string { return s })
	mapStr("to_str", func(s string) string { return s })
	mapStr("inspect", object.QuoteString)
	mapStr("squeeze", func(s string) string {
		var b strings.Builder
		var last rune = -1
		for _, r := range s {
			if r != last {
				b.WriteRune(r)
			}
			last = r
		}
		return b.String()
	})
	def(c, "to_sym", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol(str(self)), nil
	})
	def(c, "intern", 0, c.Methods["to_sym"].Native)
	def(c, "to_i", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 1 {
			base, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			i, ok := parseIntBase(strings.TrimSpace(str(self)), int(base))
			if !ok {
				return object.Int(0), nil
			}
			return object.Int(i), nil
		}
		return object.Int(leadingInt(str(self))), nil
	})
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(leadingFloat(str(self))), nil
	})
	def(c, "hex", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, _ := parseIntBase(strings.TrimPrefix(strings.TrimSpace(str(self)), "0x"), 16)
		return object.Int(i), nil
	})
	def(c, "oct", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, _ := parseIntBase(strings.TrimSpace(str(self)), 8)
		return object.Int(i), nil
	})
	def(c, "ord", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if str(self) == "" {
			return object.Value{}, ctx.Raise(object.ArgumentError, "empty string")
		}
		r, _ := utf8.DecodeRuneInString(str(self))
		return object.Int(r), nil
	})
	def(c, "chr", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if str(self) == "" {
			return object.String(""), nil
		}
		r, _ := utf8.DecodeRuneInString(str(self))
		return object.String(string(r)), nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "chars", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := []object.Value{}
		for _, r := range str(self) {
			elems = append(elems, object.String(string(r)))
		}
		return object.NewList(elems...), nil
	})
	def(c, "each_char", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each_char"); err != nil {
			return object.Value{}, err
		}
		for _, r := range str(self) {
			if _, err := ctx.Yield(block, object.String(string(r))); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "bytes", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := make([]object.Value, len(str(self)))
		for i := 0; i < len(str(self)); i++ {
			elems[i] = object.Int(str(self)[i])
		}
		return object.NewList(elems...), nil
	})
	def(c, "codepoints", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := []object.Value{}
		for _, r := range str(self) {
			elems = append(elems, object.Int(r))
		}
		return object.NewList(elems...), nil
	})
	// byteindex is the byte offset of a substring at or after offset, which
	// must fall on a character boundary.
	def(c, "byteindex", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		sub, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s, off := str(self), int64(0)
		if len(args) == 2 {
			if off, err = wantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
		}
		if off < 0 {
			off += int64(len(s))
		}
		if off < 0 || off > int64(len(s)) {
			return object.Nil, nil
		}
		if off < int64(len(s)) && !utf8.RuneStart(s[off]) {
			return object.Value{}, ctx.Raise(object.IndexError, "offset %d does not land on character boundary", off)
		}
		i := strings.Index(s[off:], sub)
		if i < 0 {
			return object.Nil, nil
		}
		return object.Int(off + int64(i)), nil
	})
	// -@ and +@ return the string itself: every string is immutable, so the
	// deduplicated and the unfrozen copy are both the string.
	def(c, "-@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "+@", 0, c.Methods["-@"].Native)
	def(c, "dedup", 0, c.Methods["-@"].Native)
	def(c, "lines", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(lines(str(self))...), nil
	})
	def(c, "each_line", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each_line"); err != nil {
			return object.Value{}, err
		}
		for _, l := range lines(str(self)) {
			if _, err := ctx.Yield(block, l); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	strArg := func(name string, f func(s, arg string) object.Value) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			arg, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return f(str(self), arg), nil
		})
	}
	strArg("include?", func(s, a string) object.Value { return object.Bool(strings.Contains(s, a)) })
	strArg("end_with?", func(s, a string) object.Value { return object.Bool(strings.HasSuffix(s, a)) })
	strArg("delete_prefix", func(s, a string) object.Value { return object.String(strings.TrimPrefix(s, a)) })
	strArg("delete_suffix", func(s, a string) object.Value { return object.String(strings.TrimSuffix(s, a)) })
	strArg("count", func(s, a string) object.Value {
		n := 0
		for _, r := range s {
			if strings.ContainsRune(a, r) {
				n++
			}
		}
		return object.Int(n)
	})
	strArg("casecmp?", func(s, a string) object.Value { return object.Bool(strings.EqualFold(s, a)) })
	strArg("casecmp", func(s, a string) object.Value {
		return object.Int(strings.Compare(strings.ToLower(s), strings.ToLower(a)))
	})
	def(c, "tr", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		from, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		to, err := wantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		return object.String(translate(str(self), from, to)), nil
	})
	pad := func(name string, place func(s, fill string, n int) string) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, argError(ctx, len(args), "1..2")
			}
			n, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			fill := " "
			if len(args) == 2 {
				if fill, err = wantString(ctx, args[1]); err != nil {
					return object.Value{}, err
				}
				if fill == "" {
					return object.Value{}, ctx.Raise(object.ArgumentError, "zero width padding")
				}
			}
			return object.String(place(str(self), fill, int(n))), nil
		})
	}
	pad("ljust", func(s, fill string, n int) string { return s + padding(fill, n-utf8.RuneCountInString(s)) })
	pad("rjust", func(s, fill string, n int) string { return padding(fill, n-utf8.RuneCountInString(s)) + s })
	pad("center", func(s, fill string, n int) string {
		total := n - utf8.RuneCountInString(s)
		if total <= 0 {
			return s
		}
		return padding(fill, total/2) + s + padding(fill, total-total/2)
	})
	def(c, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := format(ctx, str(self), args)
		return object.String(s), err
	})
	def(c, "unicode_normalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "ascii_only?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for i := 0; i < len(str(self)); i++ {
			if str(self)[i] >= 0x80 {
				return object.False, nil
			}
		}
		return object.True, nil
	})
	def(c, "b", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bytes([]byte(str(self))), nil
	})

}

func padding(fill string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(fill)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(runes[i%len(runes)])
	}
	return b.String()
}

func lines(s string) []object.Value {
	out := []object.Value{}
	for s != "" {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, object.String(s))
			break
		}
		out = append(out, object.String(s[:i+1]))
		s = s[i+1:]
	}
	return out
}

// substitute is sub and gsub with a string pattern, a replacement string or
// map, or a block. limit is the number of replacements, -1 for all.
// translate is String#tr with character lists and a-z ranges.
func translate(s, from, to string) string {
	expand := func(spec string) []rune {
		runes := []rune(spec)
		var out []rune
		for i := 0; i < len(runes); i++ {
			if i+2 < len(runes) && runes[i+1] == '-' {
				for r := runes[i]; r <= runes[i+2]; r++ {
					out = append(out, r)
				}
				i += 2
				continue
			}
			out = append(out, runes[i])
		}
		return out
	}
	negate := strings.HasPrefix(from, "^") && len(from) > 1
	if negate {
		from = from[1:]
	}
	f, t := expand(from), expand(to)
	var b strings.Builder
	for _, r := range s {
		idx := -1
		for i, fr := range f {
			if fr == r {
				idx = i
				break
			}
		}
		if negate {
			if idx >= 0 {
				b.WriteRune(r)
			} else if len(t) > 0 {
				b.WriteRune(t[len(t)-1])
			}
			continue
		}
		switch {
		case idx < 0:
			b.WriteRune(r)
		case len(t) == 0:
		case idx < len(t):
			b.WriteRune(t[idx])
		default:
			b.WriteRune(t[len(t)-1])
		}
	}
	return b.String()
}

// stringIndex is String#[] with an index, a start and length, a range, or a
// substring.
func stringIndex(ctx object.Context, s string, args []object.Value) (object.Value, error) {
	runes := []rune(s)
	n := len(runes)
	switch len(args) {
	case 1:
		switch args[0].Kind() {
		case object.KindInt:
			a := args[0].Int()
			i := int(a)
			if i < 0 {
				i += n
			}
			if i < 0 || i >= n {
				return object.Nil, nil
			}
			return object.String(string(runes[i])), nil
		case object.KindString:
			a := args[0].Str()
			if strings.Contains(s, string(a)) {
				return object.String(a), nil
			}
			return object.Nil, nil
		case object.KindRange:
			a := args[0].Range()
			start, length, ok := rangeBounds(ctx, a, n)
			if !ok {
				return object.Nil, nil
			}
			return object.String(string(runes[start : start+length])), nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(args[0]))
	case 2:
		start, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		length, err := wantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
		}
		if start < 0 {
			start += int64(n)
		}
		if start < 0 || start > int64(n) || length < 0 {
			return object.Nil, nil
		}
		end := start + length
		if end > int64(n) {
			end = int64(n)
		}
		return object.String(string(runes[start:end])), nil
	}
	return object.Value{}, argError(ctx, len(args), "1..2")
}

// rangeBounds resolves a range over a sequence of length n into a start and
// a length, false when it is out of bounds.
func rangeBounds(ctx object.Context, r *object.Range, n int) (start, length int, ok bool) {
	lo, hi := 0, n-1
	if !r.Lo.IsZero() {
		v, isInt := r.Lo.AsInt()
		if !isInt {
			return 0, 0, false
		}
		lo = int(v)
	}
	if !r.Hi.IsZero() {
		v, isInt := r.Hi.AsInt()
		if !isInt {
			return 0, 0, false
		}
		hi = int(v)
		if r.Exclusive {
			hi--
		}
	}
	if lo < 0 {
		lo += n
	}
	if hi < 0 {
		hi += n
	}
	if lo < 0 || lo > n {
		return 0, 0, false
	}
	if hi >= n {
		hi = n - 1
	}
	if hi < lo {
		return lo, 0, true
	}
	return lo, hi - lo + 1, true
}

func parseIntBase(s string, base int) (int64, bool) {
	s = strings.ReplaceAll(s, "_", "")
	var v int64
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	for _, r := range strings.ToLower(s) {
		var d int
		switch {
		case r >= '0' && r <= '9':
			d = int(r - '0')
		case r >= 'a' && r <= 'z':
			d = int(r-'a') + 10
		default:
			return 0, false
		}
		if d >= base {
			return 0, false
		}
		v = v*int64(base) + int64(d)
	}
	if neg {
		v = -v
	}
	return v, true
}
