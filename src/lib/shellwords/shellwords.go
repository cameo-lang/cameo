// Package shellwords is Ruby's shellwords library, written in cameo:
// shellwords.cameo defines Shellwords. The Go part defines
// String#shellsplit, String#shellescape and Array#shelljoin, which a
// script cannot, and calls the module.
package shellwords

import (
	_ "embed"

	"github.com/cameo-lang/cameo/src/object"
)

// Source is the library, which require runs at the top level.
//
//go:embed shellwords.cameo
var Source string

// Install defines the String and Array methods. lib.Install calls it once.
func Install() {
	forward := func(name string) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) != 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected 0)", len(args))
			}
			m, ok := ctx.Const("Shellwords")
			if !ok {
				return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method '%s': require \"shellwords\" first", name)
			}
			return ctx.Call(m, name, self)
		}
	}
	object.StringClass.Define("shellsplit", 0, forward("shellsplit"))
	object.StringClass.Define("shellescape", 0, forward("shellescape"))
	object.ListClass.Define("shelljoin", 0, forward("shelljoin"))
}
