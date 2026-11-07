// Package lib holds Ruby's standard library as cameo builds it in: one Go
// package per library, under src/lib/<require path>, each with an Install
// that defines its classes and methods. doc/stdlib.md lists every library
// of Ruby's with its status, and how one is built in.
package lib

import (
	"github.com/cameo-lang/cameo/src/lib/abbrev"
	"github.com/cameo-lang/cameo/src/lib/base64"
	"github.com/cameo-lang/cameo/src/lib/benchmark"
	"github.com/cameo-lang/cameo/src/lib/bigdecimal"
	"github.com/cameo-lang/cameo/src/lib/cgi"
	"github.com/cameo-lang/cameo/src/lib/csv"
	"github.com/cameo-lang/cameo/src/lib/date"
	"github.com/cameo-lang/cameo/src/lib/delegate"
	"github.com/cameo-lang/cameo/src/lib/digest"
	"github.com/cameo-lang/cameo/src/lib/forwardable"
	"github.com/cameo-lang/cameo/src/lib/getoptlong"
	"github.com/cameo-lang/cameo/src/lib/io/fileio"
	"github.com/cameo-lang/cameo/src/lib/ipaddr"
	"github.com/cameo-lang/cameo/src/lib/json"
	"github.com/cameo-lang/cameo/src/lib/logger"
	"github.com/cameo-lang/cameo/src/lib/marshal"
	"github.com/cameo-lang/cameo/src/lib/matrix"
	"github.com/cameo-lang/cameo/src/lib/minispec"
	"github.com/cameo-lang/cameo/src/lib/optparse"
	"github.com/cameo-lang/cameo/src/lib/pathname"
	"github.com/cameo-lang/cameo/src/lib/prime"
	"github.com/cameo-lang/cameo/src/lib/rack"
	"github.com/cameo-lang/cameo/src/lib/securerandom"
	"github.com/cameo-lang/cameo/src/lib/shellwords"
	"github.com/cameo-lang/cameo/src/lib/singleton"
	"github.com/cameo-lang/cameo/src/lib/stringio"
	"github.com/cameo-lang/cameo/src/lib/strscan"
	"github.com/cameo-lang/cameo/src/lib/time"
	"github.com/cameo-lang/cameo/src/lib/tsort"
	unicodenormalize "github.com/cameo-lang/cameo/src/lib/unicode_normalize"
	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/lib/zlib"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/readline"
)

// Libraries are the names require accepts, each with the libraries it
// loads along with it: require "cgi" loads cgi/escape and cgi/util too.
// Every library is installed when the interpreter starts, so its classes
// exist before the require; require marks the name loaded and returns
// true the first time and false after, as Ruby's does. A library the
// core installs, under src/stdlib, is registered here as well; the
// evaluator's own, debug and cameo/runner, are its.
var Libraries = map[string][]string{
	"base64":          nil,
	"benchmark":       nil,
	"bigdecimal":      {"bigdecimal/util", "bigdecimal/math"},
	"bigdecimal/math": {"bigdecimal", "bigdecimal/util"},
	"bigdecimal/util": {"bigdecimal", "bigdecimal/math"},
	"cgi":             {"cgi/escape", "cgi/util"},
	"cgi/escape":      nil,
	"cgi/util":        {"cgi/escape"},
	"csv":             nil,
	"date":            nil,
	"delegate":        nil,
	"digest":          {"digest/md5", "digest/sha1", "digest/sha2"},
	"digest/md5":      {"digest"},
	"digest/sha1":     {"digest"},
	"digest/sha2":     {"digest"},
	"forwardable":     nil,
	"ipaddr":          nil,
	"json":            nil,
	"logger":          nil,
	"marshal":         nil,
	"matrix":          nil,
	"minispec":        nil,
	// openssl is accepted for the OpenSSL::HMAC and OpenSSL::Digest names
	// digest defines; a script written for openssl begins with it.
	"openssl":                     {"digest"},
	"optparse":                    nil,
	"pathname":                    nil,
	"pp":                          nil,
	"prime":                       nil,
	"random/formatter":            nil,
	"securerandom":                {"random/formatter"},
	"set":                         nil,
	"singleton":                   nil,
	"stringio":                    nil,
	"strscan":                     nil,
	"time":                        nil,
	"unicode_normalize":           {"unicode_normalize/normalize"},
	"unicode_normalize/normalize": nil,
	"uri":                         nil,
	"zlib":                        nil,
}

// IOKinds names, for each library that does I/O, the kind it does:
// network, file or popen. Such a library acts only in a program that
// allowed its kind, or it by name, through cameo.Program.AllowIO; nothing
// is allowed unless the program says so.
var IOKinds = map[string]string{}

// Sources are the libraries written in cameo, by the name require
// accepts: the source of each, which a library's Go package embeds and
// registers here. require parses it and runs it at the top level of the
// script the first time, after marking the name loaded; its classes and
// methods are the script's from then on, as a Go library's are from the
// start. A library listed here needs no entry in Libraries, and may have
// one for what it loads along with it.
var Sources = map[string]string{
	"rack":       rack.Source,
	"getoptlong": getoptlong.Source,
	"abbrev":     abbrev.Source,
	"shellwords": shellwords.Source,
	"tsort":      tsort.Source,
}

// TopLevel are the modules a library includes at the top level of the
// script that requires it, by the library's name: the functions a script
// calls without a receiver once it required the library, which are no
// methods of every object, so that File.methods does not list them.
var TopLevel = map[string][]*object.Class{
	"minispec": minispec.TopLevel,
}

// Install defines every library. The evaluator calls it once, after
// stdlib.Install.
func Install() {
	abbrev.Install()
	base64.Install()
	benchmark.Install()
	cgi.Install()
	csv.Install()
	date.Install()
	delegate.Install()
	digest.Install()
	forwardable.Install()
	ipaddr.Install()
	json.Install()
	logger.Install()
	marshal.Install()
	pathname.Install()
	time.Install()
	minispec.Install()
	uri.Install()
	securerandom.Install()
	shellwords.Install()
	unicodenormalize.Install()
	zlib.Install()
	prime.Install()
	singleton.Install()
	stringio.Install()
	fileio.Install()
	rack.Install()
	strscan.Install()
	optparse.Install()
	getoptlong.Install()
	matrix.Install()
	bigdecimal.Install()
	readline.Install()
}
