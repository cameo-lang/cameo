// Package lib holds Ruby's standard library as cameo builds it in: one Go
// package per library, under src/lib/<require path>, each with an Install
// that defines its classes and methods. doc/stdlib.md lists every library
// of Ruby's with its status, and how one is built in.
package lib

import (
	"github.com/cameo-lang/cameo/src/lib/benchmark"
	"github.com/cameo-lang/cameo/src/lib/cgi"
	"github.com/cameo-lang/cameo/src/lib/json"
)

// Libraries are the names require accepts, each with the libraries it
// loads along with it: require "cgi" loads cgi/escape and cgi/util too.
// Every library is installed when the interpreter starts, so its classes
// exist before the require; require marks the name loaded and returns
// true the first time and false after, as Ruby's does.
var Libraries = map[string][]string{
	"benchmark":  nil,
	"cgi":        {"cgi/escape", "cgi/util"},
	"cgi/escape": nil,
	"cgi/util":   {"cgi/escape"},
	"json":       nil,
}

// Install defines every library. The evaluator calls it once, after
// stdlib.Install.
func Install() {
	benchmark.Install()
	cgi.Install()
	json.Install()
}
