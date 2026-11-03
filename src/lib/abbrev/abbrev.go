// Package abbrev is Ruby's abbrev library, built into cameo:
// Abbrev.abbrev and Array#abbrev, the unambiguous abbreviations of a set
// of words.
//
// It follows abbrev 0.1.2, the version Ruby 3.4 bundles: every prefix of
// a word that no other word shares maps to the word, the word itself
// included, and a pattern, a Regexp or a String prefix, keeps only the
// abbreviations that match it.
package abbrev

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var module = object.NewModule("Abbrev")

// Install defines Abbrev and Array#abbrev. lib.Install calls it once.
func Install() {
	m := module
	object.ObjectClass.Consts["Abbrev"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("0.1.2")
	abbrev := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		list, err := stdlib.ToList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		words := make([]string, len(list))
		for i, w := range list {
			if words[i], err = stdlib.WantString(ctx, w); err != nil {
				return object.Value{}, err
			}
		}
		// matches says whether an abbreviation passes the pattern: a
		// String is a prefix, a Regexp is matched.
		matches := func(s string) bool { return true }
		if len(args) == 2 && !args[1].IsNil() {
			if prefix, ok := args[1].AsString(); ok {
				matches = func(s string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
			} else if re, ok := args[1].AsRegexp(); ok {
				matches = func(s string) bool { return re.MatchAt(s, 0) != nil }
			} else {
				return object.Value{}, ctx.Raise(object.TypeError, "the pattern is a String or a Regexp, not %s", stdlib.KindName(args[1]))
			}
		}
		table := object.NewMap()
		seen := map[string]int{}
		for _, word := range words {
			if word == "" {
				continue
			}
			runes := []rune(word)
			for n := len(runes); n >= 1; n-- {
				ab := string(runes[:n])
				if !matches(ab) {
					continue
				}
				seen[ab]++
				switch seen[ab] {
				case 1:
					table = table.Put(object.String(ab), object.String(word))
				case 2:
					table = table.Without(object.String(ab))
				default:
					n = 0 // Ruby breaks out of the word here
				}
			}
		}
		for _, word := range words {
			if !matches(word) {
				continue
			}
			table = table.Put(object.String(word), object.String(word))
		}
		return object.MapValue(table), nil
	}
	m.DefineSingleton("abbrev", -1, abbrev)
	m.Define("abbrev", -1, abbrev)
	object.ListClass.Define("abbrev", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return abbrev(ctx, self, append([]object.Value{self}, args...), kw, block)
	})
}
