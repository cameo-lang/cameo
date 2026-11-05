// Package shellwords is Ruby's shellwords library, written in cameo:
// shellwords.cameo defines Shellwords. The Go part defines
// String#shellsplit, String#shellescape and Array#shelljoin, which a
// script cannot, and calls the module. They exist before the require, as
// every Go-defined method does, and say to require the library until it
// is loaded.
package shellwords

import (
	_ "embed"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Source is the library, which require runs at the top level.
//
//go:embed shellwords.cameo
var Source string

// Install defines the String and Array methods, which call the module.
// lib.Install calls it once.
func Install() {
	object.StringClass.Define("shellsplit", 0, stdlib.Forward("Shellwords", "shellsplit", "shellwords"))
	object.StringClass.Define("shellescape", 0, stdlib.Forward("Shellwords", "shellescape", "shellwords"))
	object.ListClass.Define("shelljoin", 0, stdlib.Forward("Shellwords", "shelljoin", "shellwords"))
}
