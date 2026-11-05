// Package base64 is Ruby's base64 library, built into cameo: encode64 and
// decode64, their strict forms, and the URL-safe ones.
//
// It follows base64 0.2.0, the version Ruby 3.4 ships: encode64 breaks
// its output into lines of sixty characters, decode64 skips what is not
// base64, and the strict and URL-safe forms refuse it with Ruby's
// ArgumentError. What is encoded is a US-ASCII String, what is decoded a
// binary one, as in Ruby. Ruby's methods are pack("m") and unpack("m") by
// other names, and so are cameo's: the coding is stdlib's, shared with
// pack and unpack.
package base64

import (
	"encoding/base64"
	"slices"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module = object.NewModule("Base64")
	// toStandard turns the URL-safe alphabet into the standard one.
	toStandard = strings.NewReplacer("-", "+", "_", "/")
)

// Install defines Base64. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Base64", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.2.0")
	// def defines a method on the module and as a module function, taking
	// the string and, where keywords is set, the keywords it allows; a
	// keyword elsewhere is one argument too many, as it is in Ruby.
	def := func(name string, keywords []string, f func(ctx object.Context, s string, kw *object.Map) (object.Value, error)) {
		fn := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if kw != nil && kw.Len() > 0 {
				if keywords == nil {
					return object.Value{}, stdlib.ArgError(ctx, len(args)+1, "1")
				}
				for _, k := range kw.Keys() {
					if !slices.Contains(keywords, k.Str()) {
						s, err := ctx.Inspect(k)
						if err != nil {
							return object.Value{}, err
						}
						return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
					}
				}
			}
			if len(args) != 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, s, kw)
		}
		m.DefineSingleton(name, -1, fn)
		m.Define(name, -1, fn)
	}
	ascii := func(s string) object.Value { return object.StringOf(object.KindASCII, s) }
	def("encode64", nil, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return ascii(stdlib.Base64Lines(base64.StdEncoding.EncodeToString([]byte(s)), 60)), nil
	})
	def("strict_encode64", nil, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return ascii(base64.StdEncoding.EncodeToString([]byte(s))), nil
	})
	def("urlsafe_encode64", []string{"padding"}, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
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
	def("decode64", nil, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		return object.Bytes(stdlib.DecodeBase64(s)), nil
	})
	def("strict_decode64", nil, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		b, ok := stdlib.DecodeBase64Strict(s)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid base64")
		}
		return object.Bytes(b), nil
	})
	def("urlsafe_decode64", nil, func(ctx object.Context, s string, kw *object.Map) (object.Value, error) {
		// Ruby turns the URL-safe alphabet into the standard one, pads what
		// lacks padding, then decodes strictly, so + and / pass as well.
		s = toStandard.Replace(s)
		if !strings.HasSuffix(s, "=") && len(s)%4 != 0 {
			s += strings.Repeat("=", 4-len(s)%4)
		}
		b, ok := stdlib.DecodeBase64Strict(s)
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid base64")
		}
		return object.Bytes(b), nil
	})
}
