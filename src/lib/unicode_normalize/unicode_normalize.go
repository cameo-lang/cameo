// Package unicode_normalize is Ruby's unicode_normalize, built into cameo:
// String#unicode_normalize and unicode_normalized? with the forms :nfc,
// :nfd, :nfkc and :nfkd, on golang.org/x/text/unicode/norm.
//
// Ruby loads the library itself when a script calls the methods, so they
// are there without a require; require "unicode_normalize" is accepted.
// unicode_normalize! is a bang method cameo has not. A binary String
// raises Encoding::CompatibilityError, as in Ruby.
package unicode_normalize

import (
	"golang.org/x/text/unicode/norm"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var forms = map[string]norm.Form{"nfc": norm.NFC, "nfd": norm.NFD, "nfkc": norm.NFKC, "nfkd": norm.NFKD}

// Install defines the String methods. lib.Install calls it once.
func Install() {
	// form reads the normalization form argument: a Symbol, :nfc when
	// absent.
	form := func(ctx object.Context, args []object.Value) (norm.Form, error) {
		if len(args) > 1 {
			return 0, stdlib.ArgError(ctx, len(args), "0..1")
		}
		if len(args) == 0 {
			return norm.NFC, nil
		}
		name, ok := args[0].AsSymbol()
		if f, known := forms[name]; ok && known {
			return f, nil
		}
		shown, err := ctx.ToS(args[0])
		if err != nil {
			return 0, err
		}
		return 0, ctx.Raise(object.ArgumentError, "Invalid normalization form %s.", shown)
	}
	// unicode reads the receiver as Unicode text: a binary String is
	// refused as Ruby refuses it.
	unicode := func(ctx object.Context, self object.Value) (string, error) {
		if self.Kind() == object.KindBytes {
			return "", ctx.Raise(object.CompatibilityError, "Unicode Normalization not appropriate for ASCII-8BIT")
		}
		return self.Str(), nil
	}
	object.StringClass.Define("unicode_normalize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := unicode(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		f, err := form(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.StringOf(self.Kind(), f.String(s)), nil
	})
	object.StringClass.Define("unicode_normalized?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := unicode(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		f, err := form(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(f.IsNormalString(s)), nil
	})
}
