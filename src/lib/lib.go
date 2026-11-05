// Package lib holds Ruby's standard library as cameo builds it in: one Go
// package per library, under src/lib/<require path>, each with an Install
// that defines its classes and methods. doc/stdlib.md lists every library
// of Ruby's with its status, and how one is built in.
package lib

import (
	"github.com/cameo-lang/cameo/src/lib/base64"
	"github.com/cameo-lang/cameo/src/lib/benchmark"
	"github.com/cameo-lang/cameo/src/lib/cgi"
	"github.com/cameo-lang/cameo/src/lib/date"
	"github.com/cameo-lang/cameo/src/lib/digest"
	"github.com/cameo-lang/cameo/src/lib/json"
	"github.com/cameo-lang/cameo/src/lib/minispec"
	"github.com/cameo-lang/cameo/src/lib/pathname"
	"github.com/cameo-lang/cameo/src/lib/time"
	"github.com/cameo-lang/cameo/src/lib/uri"
)

// Libraries are the names require accepts, each with the libraries it
// loads along with it: require "cgi" loads cgi/escape and cgi/util too.
// Every library is installed when the interpreter starts, so its classes
// exist before the require; require marks the name loaded and returns
// true the first time and false after, as Ruby's does. A library the
// core installs, under src/stdlib, is registered here as well; the
// evaluator's own, debug and cameo/runner, are its.
var Libraries = map[string][]string{
	"base64":      nil,
	"benchmark":   nil,
	"cgi":         {"cgi/escape", "cgi/util"},
	"cgi/escape":  nil,
	"cgi/util":    {"cgi/escape"},
	"date":        nil,
	"digest":      {"digest/md5", "digest/sha1", "digest/sha2"},
	"digest/md5":  {"digest"},
	"digest/sha1": {"digest"},
	"digest/sha2": {"digest"},
	"json":        nil,
	"minispec":    nil,
	// openssl is accepted for the OpenSSL::HMAC and OpenSSL::Digest names
	// digest defines; a script written for openssl begins with it.
	"openssl":  {"digest"},
	"pathname": nil,
	"pp":       nil,
	"set":      nil,
	"time":     nil,
	"uri":      nil,
}

// Sources are the libraries written in cameo, by the name require
// accepts: the source of each, which a library's Go package embeds and
// registers here. require parses it and runs it at the top level of the
// script the first time, after marking the name loaded; its classes and
// methods are the script's from then on, as a Go library's are from the
// start. A library listed here needs no entry in Libraries, and may have
// one for what it loads along with it.
var Sources = map[string]string{}

// Install defines every library. The evaluator calls it once, after
// stdlib.Install.
func Install() {
	base64.Install()
	benchmark.Install()
	cgi.Install()
	date.Install()
	digest.Install()
	json.Install()
	pathname.Install()
	time.Install()
	minispec.Install()
	uri.Install()
}
