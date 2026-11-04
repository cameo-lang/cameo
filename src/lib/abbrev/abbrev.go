// Package abbrev is Ruby's abbrev library, written in cameo: abbrev.cameo
// defines Abbrev. The Go part defines Array#abbrev, which a script cannot,
// and calls the module.
package abbrev

import (
	_ "embed"

	"github.com/cameo-lang/cameo/src/object"
)

// Source is the library, which require runs at the top level.
//
//go:embed abbrev.cameo
var Source string

// Install defines Array#abbrev: Abbrev.abbrev(self, pattern). lib.Install
// calls it once.
func Install() {
	object.ListClass.Define("abbrev", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m, ok := ctx.Const("Abbrev")
		if !ok {
			return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method 'abbrev' for an instance of Array: require \"abbrev\" first")
		}
		return ctx.Call(m, "abbrev", append([]object.Value{self}, args...)...)
	})
}
