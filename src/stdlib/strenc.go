package stdlib

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/radiospiel/amber/src/object"
)

// The String methods are written for UTF-8. installStringEncodings wraps
// each so that a binary or US-ASCII string behaves as Ruby's does: one
// character per byte, case changed in ASCII alone, and a result in the
// receiver's encoding. A UTF-8 string with UTF-8 arguments goes straight
// to the method.
//
// Most methods see a binary string through latin1: each byte as the
// character of the same number, so that UTF-8 code counting characters
// counts bytes; the result comes back the same way. Methods that take a
// pattern instead see the bytes as they are, so that a pattern over bytes
// matches them.

// ownEncoding are the methods that read the receiver's encoding
// themselves, or do not depend on it.
var ownEncoding = map[string]bool{
	"encoding": true, "force_encoding": true, "b": true, "valid_encoding?": true, "scrub": true,
	"encode": true, "unicode_normalize": true, "bytes": true, "bytesize": true, "byteindex": true,
	"unpack": true, "unpack1": true, "hash": true, "to_sym": true, "intern": true, "to_i": true,
	"to_f": true, "hex": true, "oct": true, "==": true, "eql?": true, "===": true, "<=>": true,
	"empty?": true, "ascii_only?": true, "frozen?": true, "-@": true, "+@": true, "dedup": true,
	"to_s": true, "to_str": true, "include?": true, "start_with?": true, "end_with?": true,
}

// patternMethods take a string or a regexp to match the bytes with.
var patternMethods = map[string]bool{
	"sub": true, "gsub": true, "scan": true, "match": true, "match?": true, "=~": true,
	"partition": true, "rpartition": true,
}

func toLatin1(s string) string {
	if object.ASCIIOnly(s) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteRune(rune(s[i]))
	}
	return b.String()
}

func fromLatin1(s string) string {
	if object.ASCIIOnly(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 0x100 {
			b.WriteByte(byte(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// relabel gives the strings a method returned the receiver's encoding,
// through f, in lists too.
func relabel(v object.Value, k object.Kind, f func(string) string) object.Value {
	switch {
	case v.IsString():
		return object.StringOf(k, f(v.Str()))
	case v.Kind() == object.KindList:
		elems := v.List().Elems
		out := make([]object.Value, len(elems))
		for i, e := range elems {
			out[i] = relabel(e, k, f)
		}
		return object.NewList(out...)
	}
	return v
}

func viewArgs(args []object.Value) []object.Value {
	out := make([]object.Value, len(args))
	for i, a := range args {
		switch {
		case a.IsString():
			out[i] = object.String(toLatin1(a.Str()))
		case a.Kind() == object.KindList:
			out[i] = object.NewList(viewArgs(a.List().Elems)...)
		default:
			out[i] = a
		}
	}
	return out
}

func hasRegexp(args []object.Value) bool {
	for _, a := range args {
		if _, ok := a.AsRegexp(); ok {
			return true
		}
	}
	return false
}

func installStringEncodings() {
	c := object.StringClass
	var names []string
	for name, m := range c.Methods {
		if m.Native != nil && !ownEncoding[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := c.Methods[name]
		fn, name := m.Native, name
		c.Define(name, m.Arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			k := self.Kind()
			if k == object.KindString {
				return fn(ctx, self, args, kw, block)
			}
			s := self.Str()
			switch {
			case patternMethods[name] || hasRegexp(args):
				v, err := fn(ctx, self, args, kw, block)
				return relabel(v, k, func(s string) string { return s }), err
			case name == "inspect" || name == "dump":
				return object.String(object.QuoteBytes([]byte(s))), nil
			}
			view := object.String(toLatin1(s))
			v, err := fn(ctx, view, viewArgs(args), kw, block)
			if err != nil {
				return object.Value{}, err
			}
			return relabel(v, k, fromLatin1), nil
		})
	}
	// Case changes in ASCII alone outside UTF-8.
	asciiOnly := func(name string, f func(string) string) {
		utf := c.Methods[name].Native
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if self.Kind() == object.KindString {
				return utf(ctx, self, args, kw, block)
			}
			return object.StringOf(self.Kind(), f(self.Str())), nil
		})
	}
	asciiOnly("upcase", func(s string) string { return asciiCase(s, asciiUpper) })
	asciiOnly("downcase", func(s string) string { return asciiCase(s, asciiLower) })
	asciiOnly("swapcase", func(s string) string {
		return asciiCase(s, func(c byte) byte {
			if u := asciiUpper(c); u != c {
				return u
			}
			return asciiLower(c)
		})
	})
	asciiOnly("capitalize", func(s string) string {
		low := asciiCase(s, asciiLower)
		if low == "" {
			return low
		}
		return string(asciiUpper(low[0])) + low[1:]
	})
	// each_char and each_line yield in the receiver's encoding.
	// Without a block each_char is an enumerator of the characters.
	def(c, "each_char", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k, s := self.Kind(), self.Str()
		chars := func(ctx object.Context, yield func(object.Value) error) error {
			for i := 0; i < len(s); {
				size := 1
				if k == object.KindString {
					_, size = utf8.DecodeRuneInString(s[i:])
				}
				if err := yield(object.StringOf(k, s[i:i+size])); err != nil {
					return err
				}
				i += size
			}
			return nil
		}
		if block.IsZero() {
			return enumerator(false, self, "each_char", nil, chars), nil
		}
		err := chars(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		if err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	eachLine := c.Methods["each_line"].Native
	def(c, "each_line", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k := self.Kind()
		if k == object.KindString || block.IsZero() {
			return eachLine(ctx, self, args, kw, block)
		}
		for _, line := range strings.SplitAfter(self.Str(), "\n") {
			if line == "" {
				continue
			}
			if _, err := ctx.Yield(block, object.StringOf(k, line)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	// + makes a string of the encoding both are compatible in.
	plus := c.Methods["+"].Native
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !args[0].IsString() || self.Kind() == object.KindString && args[0].Kind() == object.KindString {
			return plus(ctx, self, args, kw, block)
		}
		k, ok := compatible(self, args[0])
		if !ok {
			return object.Value{}, incompatible(ctx, self, args[0])
		}
		return object.StringOf(k, self.Str()+args[0].Str()), nil
	})
}
