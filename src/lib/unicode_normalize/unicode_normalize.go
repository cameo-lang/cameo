// Package unicode_normalize is Ruby's unicode_normalize, built into cameo:
// String#unicode_normalize and unicode_normalized? with the forms :nfc,
// :nfd, :nfkc and :nfkd, on golang.org/x/text/unicode/norm.
//
// Ruby loads the library itself when a script calls the methods, so they
// are there without a require; require "unicode_normalize" is accepted,
// where Ruby knows only unicode_normalize/normalize. unicode_normalize! is
// a bang method cameo has not. A binary String raises
// Encoding::CompatibilityError, a UTF-8 String with a byte sequence that
// is not UTF-8 raises ArgumentError, and a US-ASCII String is as it is,
// as in Ruby.
package unicode_normalize

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var forms = map[string]norm.Form{"nfc": norm.NFC, "nfd": norm.NFD, "nfkc": norm.NFKC, "nfkd": norm.NFKD}

// Install defines the String methods. lib.Install calls it once.
func Install() {
	object.StringClass.Define("unicode_normalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, err := form(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		s, text, err := unicode(ctx, self)
		if err != nil || !text {
			return self, err
		}
		return object.NewString(f.String(s), object.EncodingOf(self.Kind())), nil
	})
	object.StringClass.Define("unicode_normalized?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, err := form(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		s, text, err := unicode(ctx, self)
		if err != nil || !text {
			return object.Bool(err == nil), err
		}
		return object.Bool(f.IsNormalString(s)), nil
	})
}

// form reads the normalization form argument: a Symbol, :nfc when absent.
func form(ctx object.Context, args []object.Value) (norm.Form, error) {
	if len(args) > 1 {
		return 0, stdlib.ArgError(ctx, len(args), "0..1")
	}
	if len(args) == 0 {
		return norm.NFC, nil
	}
	if name, ok := args[0].AsSymbol(); ok {
		if f, known := forms[name]; known {
			return f, nil
		}
	}
	shown, err := ctx.ToS(args[0])
	if err != nil {
		return 0, err
	}
	return 0, ctx.Raise(object.ArgumentError, "Invalid normalization form %s.", shown)
}

// unicode reads the receiver as Unicode text, which text says it is: a
// US-ASCII String is not normalized, as Ruby leaves it, a binary one is
// refused, and a UTF-8 one must be UTF-8.
func unicode(ctx object.Context, self object.Value) (s string, text bool, err error) {
	switch self.Kind() {
	case object.KindBytes:
		return "", false, ctx.Raise(object.CompatibilityError, "Unicode Normalization not appropriate for ASCII-8BIT")
	case object.KindASCII:
		return "", false, nil
	}
	s = self.Str()
	if !utf8.ValidString(s) {
		return "", false, ctx.Raise(object.ArgumentError, "invalid byte sequence in UTF-8")
	}
	return s, true, nil
}
