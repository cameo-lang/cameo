package stdlib

import (
	"github.com/cameo-lang/cameo/src/internal/trusted"
	"github.com/cameo-lang/cameo/src/str"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

func installString() {
	c := object.StringClass
	strOf := func(self object.Value) string { return string(self.Str()) }
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsString()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", KindName(args[0]))
		}
		return object.Joined(strOf(self)+string(o), self, args[0]), nil
	})
	def(c, "*", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative argument")
		}
		// Copies of a string that begins at a character have its
		// characters each.
		if chars := self.Chars(); chars > 0 && utf8.RuneStart(strOf(self)[0]) {
			return object.TrustedString(trusted.Key{}, strings.Repeat(strOf(self), int(n)), str.UTF8, chars*int(n)), nil
		}
		return object.NewString(strings.Repeat(strOf(self), int(n)), str.UTF8), nil
	})
	def(c, "%", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var fargs []object.Value
		if l, ok := args[0].AsList(); ok {
			fargs = l.Elems
		} else {
			fargs = args
		}
		s, err := format(ctx, strOf(self), fargs)
		return object.NewString(s, str.UTF8), err
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
		return object.Int(strings.Compare(strOf(self), string(o))), nil
	})
	def(c, "eql?", 1, c.Methods["=="].Native)
	def(c, "===", 1, c.Methods["=="].Native)
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// A string that was made with its characters counted has them.
		if n := self.Chars(); n >= 0 {
			return object.Int(n), nil
		}
		return object.Int(utf8.RuneCountInString(strOf(self))), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "bytesize", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(len(strOf(self))), nil
	})
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(strOf(self) == ""), nil
	})
	mapStr := func(name string, f func(string) string) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.NewString(f(strOf(self)), str.UTF8), nil
		})
	}
	// alike makes a method whose result has a character for each of the
	// receiver's, and trimmed one whose result is the receiver less
	// characters of one byte: the result knows its characters when the
	// receiver does.
	counted := func(name string, f func(string) string, chars func(n int, in, out string) int) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			in := strOf(self)
			out := f(in)
			if n := self.Chars(); n >= 0 {
				return object.TrustedString(trusted.Key{}, out, str.UTF8, chars(n, in, out)), nil
			}
			// A receiver without a count is of another encoding, or too
			// long to keep one: the result is of the receiver's.
			return object.NewString(out, object.EncodingOf(self.Kind())), nil
		})
	}
	alike := func(name string, f func(string) string) {
		counted(name, f, func(n int, in, out string) int { return n })
	}
	trimmed := func(name string, f func(string) string) {
		counted(name, f, func(n int, in, out string) int { return n - (len(in) - len(out)) })
	}
	alike("upcase", strings.ToUpper)
	alike("downcase", strings.ToLower)
	alike("capitalize", func(s string) string {
		if s == "" {
			return s
		}
		r, size := utf8.DecodeRuneInString(s)
		return string(unicode.ToUpper(r)) + strings.ToLower(s[size:])
	})
	alike("swapcase", func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsUpper(r) {
				return unicode.ToLower(r)
			}
			return unicode.ToUpper(r)
		}, s)
	})
	alike("reverse", func(s string) string {
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	})
	trimmed("strip", func(s string) string { return strings.Trim(s, " \t\n\r\f\v\x00") })
	trimmed("lstrip", func(s string) string { return strings.TrimLeft(s, " \t\n\r\f\v\x00") })
	trimmed("rstrip", func(s string) string { return strings.TrimRight(s, " \t\n\r\f\v\x00") })
	trimmed("chomp", func(s string) string {
		s = strings.TrimSuffix(s, "\n")
		return strings.TrimSuffix(s, "\r")
	})
	// chomp(suffix) drops that suffix, and chomp("") every line ending.
	chomp := c.Methods["chomp"].Native
	def(c, "chomp", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return chomp(ctx, self, args, kw, block)
		}
		suffix, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if suffix == "" {
			// Line endings are a byte each.
			out := strings.TrimRight(strOf(self), "\r\n")
			if n := self.Chars(); n >= 0 {
				return object.TrustedString(trusted.Key{}, out, str.UTF8, n-(len(strOf(self))-len(out))), nil
			}
			return object.NewString(out, str.UTF8), nil
		}
		return lessSuffix(self, suffix), nil
	})
	counted("chop", func(s string) string {
		if s == "" {
			return s
		}
		_, size := utf8.DecodeLastRuneInString(s)
		return s[:len(s)-size]
	}, func(n int, in, out string) int { return max(n-1, 0) })
	mapStr("succ", succ)
	mapStr("next", succ)
	// upto yields self and its successors up to max, as a range of strings
	// from self to max does.
	def(c, "upto", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, ArgError(ctx, len(args), "1..2")
		}
		hi, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		exclusive := len(args) == 2 && args[1].Truthy()
		if block.IsZero() {
			return enumFor(self, "upto", args, nil), nil
		}
		for s := range stringsUpto(self.Str(), hi, exclusive, false) {
			if _, err := ctx.Yield(block, object.NewString(s, str.UTF8)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	alike("to_s", func(s string) string { return s })
	alike("to_str", func(s string) string { return s })
	mapStr("inspect", object.QuoteString)
	def(c, "squeeze", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// The characters that stay are counted as they are written.
		var b strings.Builder
		var last rune = -1
		kept := 0
		for _, r := range strOf(self) {
			if r != last {
				b.WriteRune(r)
				kept++
			}
			last = r
		}
		return object.TrustedString(trusted.Key{}, b.String(), str.UTF8, kept), nil
	})
	def(c, "to_sym", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !validIn(self.Kind(), strOf(self)) {
			quoted := object.QuoteString(strOf(self))
			if self.Kind() == object.KindASCII {
				quoted = object.QuoteBytes([]byte(strOf(self)))
			}
			return object.Value{}, ctx.Raise(object.EncodingError, "invalid symbol in encoding %s :%s", encodingName(self.Kind()), quoted)
		}
		return object.Symbol(strOf(self)), nil
	})
	def(c, "intern", 0, c.Methods["to_sym"].Native)
	def(c, "to_i", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 1 {
			base, err := WantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			i, ok := parseIntBase(strings.TrimSpace(strOf(self)), int(base))
			if !ok {
				return object.Int(0), nil
			}
			return i, nil
		}
		return leadingInt(strOf(self)), nil
	})
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(leadingFloat(strOf(self))), nil
	})
	def(c, "hex", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if i, ok := parseIntBase(strings.TrimPrefix(strings.TrimSpace(strOf(self)), "0x"), 16); ok {
			return i, nil
		}
		return object.Int(0), nil
	})
	def(c, "oct", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if i, ok := parseIntBase(strings.TrimSpace(strOf(self)), 8); ok {
			return i, nil
		}
		return object.Int(0), nil
	})
	def(c, "ord", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s := strOf(self)
		if s == "" {
			return object.Value{}, ctx.Raise(object.ArgumentError, "empty string")
		}
		switch self.Kind() {
		case object.KindBytes:
			return object.Int(int64(s[0])), nil
		case object.KindASCII:
			if s[0] >= 0x80 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid byte sequence in US-ASCII")
			}
			return object.Int(int64(s[0])), nil
		}
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size < 2 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid byte sequence in UTF-8")
		}
		return object.Int(r), nil
	})
	def(c, "chr", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if strOf(self) == "" {
			return object.NewString("", str.ASCII), nil
		}
		r, _ := utf8.DecodeRuneInString(strOf(self))
		return object.NewString(string(r), str.UTF8), nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "chars", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := []object.Value{}
		for _, r := range strOf(self) {
			elems = append(elems, object.TrustedString(trusted.Key{}, string(r), str.UTF8, 1))
		}
		return object.NewList(elems...), nil
	})
	def(c, "each_char", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return enumFor(self, "each_char", nil, func(ctx object.Context) (object.Value, error) {
				return ctx.Call(self, "size")
			}), nil
		}
		for _, r := range strOf(self) {
			if _, err := ctx.Yield(block, object.TrustedString(trusted.Key{}, string(r), str.UTF8, 1)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "bytes", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := make([]object.Value, len(strOf(self)))
		for i := 0; i < len(strOf(self)); i++ {
			elems[i] = object.Int(strOf(self)[i])
		}
		return object.NewList(elems...), nil
	})
	def(c, "codepoints", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems := []object.Value{}
		for _, r := range strOf(self) {
			elems = append(elems, object.Int(r))
		}
		return object.NewList(elems...), nil
	})
	// byteindex is the byte offset of a substring at or after offset, which
	// must fall on a character boundary.
	def(c, "byteindex", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, ArgError(ctx, len(args), "1..2")
		}
		sub, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s, off := strOf(self), int64(0)
		if len(args) == 2 {
			if off, err = WantInt(ctx, args[1], ""); err != nil {
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
		return object.NewList(lines(strOf(self))...), nil
	})
	def(c, "each_line", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return enumFor(self, "each_line", args, nil), nil
		}
		for _, l := range lines(strOf(self)) {
			if _, err := ctx.Yield(block, l); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	strArg := func(name string, f func(s, arg string) object.Value) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			arg, err := WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return f(strOf(self), arg), nil
		})
	}
	strArg("include?", func(s, a string) object.Value { return object.Bool(strings.Contains(s, a)) })
	strArg("end_with?", func(s, a string) object.Value { return object.Bool(strings.HasSuffix(s, a)) })
	def(c, "delete_prefix", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		prefix, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s := strOf(self)
		out := strings.TrimPrefix(s, prefix)
		// The rest has the receiver's characters less the prefix's,
		// unless it begins inside a character (object.Joined).
		if n := self.Chars(); n >= 0 && (out == "" || utf8.RuneStart(out[0])) {
			return object.TrustedString(trusted.Key{}, out, str.UTF8, n-utf8.RuneCountInString(s[:len(s)-len(out)])), nil
		}
		return object.NewString(out, str.UTF8), nil
	})
	def(c, "delete_suffix", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		suffix, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return lessSuffix(self, suffix), nil
	})
	strArg("count", func(s, a string) object.Value {
		n := 0
		for _, r := range s {
			if strings.ContainsRune(a, r) {
				n++
			}
		}
		return object.Int(n)
	})
	def(c, "casecmp", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return casecmp(self, args[0]), nil
	})
	def(c, "casecmp?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return casecmpP(self, args[0]), nil
	})
	def(c, "tr", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		from, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		to, err := WantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		out, chars := translate(strOf(self), from, to)
		return object.TrustedString(trusted.Key{}, out, str.UTF8, chars), nil
	})
	pad := func(name string, place func(s, fill string, n int) string) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 || len(args) > 2 {
				return object.Value{}, ArgError(ctx, len(args), "1..2")
			}
			n, err := WantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			fill := " "
			if len(args) == 2 {
				if fill, err = WantString(ctx, args[1]); err != nil {
					return object.Value{}, err
				}
				if fill == "" {
					return object.Value{}, ctx.Raise(object.ArgumentError, "zero width padding")
				}
			}
			// The result has the receiver's characters and the
			// padding's, which the method counts to pad: unless
			// the receiver begins inside a character, which
			// padding before it could complete (object.Joined).
			s := strOf(self)
			have := self.Chars()
			if have == str.UnknownLength {
				have = utf8.RuneCountInString(s)
			}
			out := place(s, fill, max(int(n)-have, 0))
			if s == "" || utf8.RuneStart(s[0]) {
				return object.TrustedString(trusted.Key{}, out, str.UTF8, max(int(n), have)), nil
			}
			return object.NewString(out, str.UTF8), nil
		})
	}
	pad("ljust", func(s, fill string, total int) string { return s + Padding(fill, total) })
	pad("rjust", func(s, fill string, total int) string { return Padding(fill, total) + s })
	pad("center", func(s, fill string, total int) string {
		if total <= 0 {
			return s
		}
		return Padding(fill, total/2) + s + Padding(fill, total-total/2)
	})
	def(c, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := format(ctx, strOf(self), args)
		return object.NewString(s, str.UTF8), err
	})
	def(c, "ascii_only?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for i := 0; i < len(strOf(self)); i++ {
			if strOf(self)[i] >= 0x80 {
				return object.False, nil
			}
		}
		return object.True, nil
	})
	def(c, "b", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bytes([]byte(strOf(self))), nil
	})

}

// Padding is n characters of fill, repeated and cut to n, as ljust and
// rjust pad with. fill must not be empty: the callers refuse an empty one
// as Ruby does, "zero width padding".
func Padding(fill string, n int) string {
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
			out = append(out, object.NewString(s, str.UTF8))
			break
		}
		out = append(out, object.NewString(s[:i+1], str.UTF8))
		s = s[i+1:]
	}
	return out
}

// substitute is sub and gsub with a string pattern, a replacement string or
// map, or a block. limit is the number of replacements, -1 for all.
// translate is String#tr with character lists and a-z ranges.
// lessSuffix is the string v without that suffix, when it ends in it:
// with v's characters less the suffix's, when v knows its own and the
// suffix does not begin inside a character (object.Joined).
func lessSuffix(v object.Value, suffix string) object.Value {
	s := v.Str()
	out := strings.TrimSuffix(s, suffix)
	n := v.Chars()
	switch {
	case n == str.UnknownLength:
		return object.NewString(out, str.UTF8)
	case len(out) == len(s):
		return object.TrustedString(trusted.Key{}, out, str.UTF8, n)
	case utf8.RuneStart(suffix[0]):
		return object.TrustedString(trusted.Key{}, out, str.UTF8, n-utf8.RuneCountInString(suffix))
	}
	return object.NewString(out, str.UTF8)
}

// partOf makes a string of s, which is cut out of v: of ASCII when v is
// known to be.
func partOf(v object.Value, s string) object.Value {
	if v.KnownASCII() {
		return object.NewString(s, str.ASCII)
	}
	return object.NewString(s, str.UTF8)
}

// translate is String#tr, and the number of characters of its result,
// counted as they are written.
func translate(s, from, to string) (string, int) {
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
	chars := 0
	write := func(r rune) {
		b.WriteRune(r)
		chars++
	}
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
				write(r)
			} else if len(t) > 0 {
				write(t[len(t)-1])
			}
			continue
		}
		switch {
		case idx < 0:
			write(r)
		case len(t) == 0:
		case idx < len(t):
			write(t[idx])
		default:
			write(t[len(t)-1])
		}
	}
	return b.String(), chars
}

// stringIndex is String#[] with an index, a start and length, a range, or a
// substring.
func stringIndex(ctx object.Context, self object.Value, args []object.Value) (object.Value, error) {
	s := self.Str()
	if len(args) == 1 {
		switch args[0].Kind() {
		case object.KindInt:
			return charAt(self, args[0].Int()), nil
		case object.KindString, object.KindBytes, object.KindASCII:
			a := args[0].Str()
			if strings.Contains(s, string(a)) {
				return object.NewString(a, str.UTF8), nil
			}
			return object.Nil, nil
		}
	}
	// The characters of a string of ASCII alone are its bytes, which
	// are sliced as they are; any other string is decoded first.
	n := len(s)
	sub := func(from, to int) object.Value {
		return object.TrustedString(trusted.Key{}, s[from:to], str.ASCII, to-from)
	}
	if !self.KnownASCII() {
		runes := []rune(s)
		n = len(runes)
		sub = func(from, to int) object.Value {
			return object.TrustedString(trusted.Key{}, string(runes[from:to]), str.UTF8, to-from)
		}
	}
	switch len(args) {
	case 1:
		switch args[0].Kind() {
		case object.KindRange:
			a := args[0].Range()
			start, length, ok := rangeBounds(ctx, a, n)
			if !ok {
				return object.Nil, nil
			}
			return sub(start, start+length), nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", KindName(args[0]))
	case 2:
		start, err := WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		length, err := WantInt(ctx, args[1], "")
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
		return sub(int(start), int(end)), nil
	}
	return object.Value{}, ArgError(ctx, len(args), "1..2")
}

// charAt returns the character at index i of the UTF-8 string self, a
// negative i counting from the end, or nil past either end. A string
// of ASCII alone answers from its bytes at once. Any other finds the
// character through its index of characters (str.Offset), so that a
// loop over a string by index reads each character once.
func charAt(self object.Value, i int64) object.Value {
	s := self.Str()
	if self.KnownASCII() {
		if i < 0 {
			i += int64(len(s))
		}
		if i < 0 || i >= int64(len(s)) {
			return object.Nil
		}
		return object.TrustedString(trusted.Key{}, s[i:i+1], str.ASCII, 1)
	}
	n := self.Chars()
	if n == str.UnknownLength { // a string too long to keep its count: count it
		n = utf8.RuneCountInString(s)
	}
	if i < 0 {
		i += int64(n)
	}
	if i < 0 || i >= int64(n) {
		return object.Nil
	}
	at := str.Offset(s, int(i))
	if at < 0 {
		return object.Nil
	}
	return object.TrustedString(trusted.Key{}, s[at:at+str.CharSize(s, at)], str.UTF8, 1)
}

// rangeBounds resolves a range over a sequence of length n into a start and
// a length, false when it is out of bounds. A nil end, written or left
// out, is the sequence's end.
func rangeBounds(ctx object.Context, r *object.Range, n int) (start, length int, ok bool) {
	lo, hi := 0, n-1
	if !object.OrNil(r.Lo).IsNil() {
		v, isInt := r.Lo.AsInt()
		if !isInt {
			return 0, 0, false
		}
		lo = int(v)
	}
	if !object.OrNil(r.Hi).IsNil() {
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

func parseIntBase(s string, base int) (object.Value, bool) {
	s = strings.ReplaceAll(s, "_", "")
	if base < 2 || base > 36 || s == "" || s[0] == '+' {
		return object.Value{}, false
	}
	b, ok := new(big.Int).SetString(strings.ToLower(s), base)
	if !ok {
		return object.Value{}, false
	}
	return object.BigInt(b), true
}

// casecmp is String#casecmp: the order of two strings with ASCII letters
// folded to lower case, nil when other is not a String or the encodings
// are incompatible.
func casecmp(self, other object.Value) object.Value {
	if !other.IsString() {
		return object.Nil
	}
	if _, ok := compatible(self, other); !ok {
		return object.Nil
	}
	a, b := self.Str(), other.Str()
	for i := 0; i < len(a) && i < len(b); i++ {
		if x, y := asciiLower(a[i]), asciiLower(b[i]); x != y {
			if x < y {
				return object.Int(-1)
			}
			return object.Int(1)
		}
	}
	switch {
	case len(a) == len(b):
		return object.Int(0)
	case len(a) > len(b):
		return object.Int(1)
	}
	return object.Int(-1)
}

// casecmpP is String#casecmp?: whether two strings are equal after case
// folding, Unicode's for UTF-8 strings and ASCII's for the others, nil
// when other is not a String or the encodings are incompatible.
func casecmpP(self, other object.Value) object.Value {
	if !other.IsString() {
		return object.Nil
	}
	if _, ok := compatible(self, other); !ok {
		return object.Nil
	}
	if self.Kind() == object.KindString && other.Kind() == object.KindString {
		return object.Bool(strings.EqualFold(self.Str(), other.Str()))
	}
	return object.Bool(asciiCase(self.Str(), asciiLower) == asciiCase(other.Str(), asciiLower))
}
