// Package abbrev is Ruby's abbrev library, written in cameo: abbrev.cameo
// defines Abbrev. The Go part defines Array#abbrev, which a script cannot,
// and calls the module; it exists before the require, as every Go-defined
// method does, and says to require the library until it is loaded.
package abbrev

import (
	_ "embed"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Source is the library, which require runs at the top level.
//
//go:embed abbrev.cameo
var Source string

// Install defines Array#abbrev: Abbrev.abbrev(self, pattern). lib.Install
// calls it once.
func Install() {
	forward := stdlib.Forward("Abbrev", "abbrev", "abbrev")
	object.ListClass.Define("abbrev", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// Keywords are the pattern, a Hash, as Ruby passes them.
		if kw != nil && kw.Len() > 0 {
			args = append(args, object.MapValue(kw))
			kw = nil
		}
		if len(args) > 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
		}
		return forward(ctx, self, args, kw, block)
	})
}
