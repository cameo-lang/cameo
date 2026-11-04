// Package base64 is Ruby's base64 library, built into cameo: encode64 and
// decode64, their strict forms, and the URL-safe ones.
//
// It follows base64 0.2.0, the version Ruby 3.4 ships: encode64 breaks
// its output into lines of sixty characters, decode64 skips what is not
// base64, and the strict and URL-safe forms refuse it with Ruby's
// ArgumentError. What is encoded is a US-ASCII String, what is decoded a
// binary one, as in Ruby.
package base64

import (
	"encoding/base64"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var module = object.NewModule("Base64")

// Install defines Base64. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Base64", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.2.0")
	def := func(name string, arity int, f func(ctx object.Context, s string, kw *object.Map) (object.Value, error)) {
		fn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) != 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, s, kw)
		}
		m.DefineSingleton(name, arity, fn)
		m.Define(name, arity, fn)
	}
	ascii := func(s string) object.Value { return object.StringOf(object.KindASCII, s) }
	def("encode64", 1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return ascii(wrap(base64.StdEncoding.EncodeToString([]byte(s)))), nil
	})
	def("strict_encode64", 1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return ascii(base64.StdEncoding.EncodeToString([]byte(s))), nil
	})
	def("urlsafe_encode64", -1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		padding := true
		if kw != nil {
			if v, ok := kw.Get(object.Symbol("padding")); ok {
				padding = v.Truthy()
			}
		}
		out := base64.URLEncoding.EncodeToString([]byte(s))
		if !padding {
			out = strings.TrimRight(out, "=")
		}
		return ascii(out), nil
	})
	def("decode64", 1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return object.Bytes(lenient(s)), nil
	})
	def("strict_decode64", 1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		b, err := base64.StdEncoding.Strict().DecodeString(s)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid base64")
		}
		return object.Bytes(b), nil
	})
	def("urlsafe_decode64", 1, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		// Ruby turns the URL-safe alphabet into the standard one, pads what
		// lacks padding, then decodes strictly, so + and / pass as well.
		s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
		if !strings.HasSuffix(s, "=") && len(s)%4 != 0 {
			s += strings.Repeat("=", 4-len(s)%4)
		}
		b, err := base64.StdEncoding.Strict().DecodeString(s)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid base64")
		}
		return object.Bytes(b), nil
	})
}

// wrap is pack("m")'s output: lines of sixty characters, each ending in a
// newline, and a newline after the last.
func wrap(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for len(s) > 60 {
		b.WriteString(s[:60])
		b.WriteByte('\n')
		s = s[60:]
	}
	b.WriteString(s)
	b.WriteByte('\n')
	return b.String()
}

// lenient is unpack1("m"): every base64 character counts, in groups of
// four, and anything else, a newline or a stray character, is skipped; a
// group cut short by the end or by padding yields the bytes it holds.
func lenient(s string) []byte {
	var out []byte
	var acc uint32
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var v byte
		switch {
		case c >= 'A' && c <= 'Z':
			v = c - 'A'
		case c >= 'a' && c <= 'z':
			v = c - 'a' + 26
		case c >= '0' && c <= '9':
			v = c - '0' + 52
		case c == '+':
			v = 62
		case c == '/':
			v = 63
		case c == '=':
			// Padding ends a group that holds two characters or more; one
			// before that is skipped like any stray character.
			if n >= 2 {
				out = flush(out, acc, n)
				acc, n = 0, 0
			}
			continue
		default:
			continue
		}
		acc = acc<<6 | uint32(v)
		n++
		if n == 4 {
			out = append(out, byte(acc>>16), byte(acc>>8), byte(acc))
			acc, n = 0, 0
		}
	}
	return flush(out, acc, n)
}

// flush writes the whole bytes of a group of fewer than four characters.
func flush(out []byte, acc uint32, n int) []byte {
	switch n {
	case 2:
		return append(out, byte(acc>>4))
	case 3:
		return append(out, byte(acc>>10), byte(acc>>2))
	}
	return out
}
