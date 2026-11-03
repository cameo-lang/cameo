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
	"github.com/cameo-lang/cameo/src/lib/ipaddr"
	"github.com/cameo-lang/cameo/src/lib/json"
	"github.com/cameo-lang/cameo/src/lib/logger"
	"github.com/cameo-lang/cameo/src/lib/marshal"
	"github.com/cameo-lang/cameo/src/lib/matrix"
	nethttp "github.com/cameo-lang/cameo/src/lib/net/http"
	openuri "github.com/cameo-lang/cameo/src/lib/open-uri"
	"github.com/cameo-lang/cameo/src/lib/optparse"
	"github.com/cameo-lang/cameo/src/lib/pathname"
	"github.com/cameo-lang/cameo/src/lib/prime"
	"github.com/cameo-lang/cameo/src/lib/rbconfig"
	"github.com/cameo-lang/cameo/src/lib/resolv"
	"github.com/cameo-lang/cameo/src/lib/securerandom"
	"github.com/cameo-lang/cameo/src/lib/shellwords"
	"github.com/cameo-lang/cameo/src/lib/singleton"
	"github.com/cameo-lang/cameo/src/lib/stringio"
	"github.com/cameo-lang/cameo/src/lib/strscan"
	libtime "github.com/cameo-lang/cameo/src/lib/time"
	"github.com/cameo-lang/cameo/src/lib/tsort"
	unicodenormalize "github.com/cameo-lang/cameo/src/lib/unicode_normalize"
	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/lib/yaml"
	"github.com/cameo-lang/cameo/src/lib/zlib"
)

// Libraries are the names require accepts, each with the libraries it
// loads along with it: require "cgi" loads cgi/escape and cgi/util too.
// Every library is installed when the interpreter starts, so its classes
// exist before the require; require marks the name loaded and returns
// true the first time and false after, as Ruby's does.
var Libraries = map[string][]string{
	"abbrev":                      nil,
	"base64":                      nil,
	"benchmark":                   nil,
	"bigdecimal":                  {"bigdecimal/util", "bigdecimal/math"},
	"bigdecimal/util":             {"bigdecimal", "bigdecimal/math"},
	"bigdecimal/math":             {"bigdecimal", "bigdecimal/util"},
	"cgi":                         {"cgi/escape", "cgi/util"},
	"cgi/escape":                  nil,
	"cgi/util":                    {"cgi/escape"},
	"csv":                         nil,
	"date":                        nil,
	"delegate":                    nil,
	"digest":                      {"digest/md5", "digest/sha1", "digest/sha2"},
	"digest/md5":                  {"digest"},
	"digest/sha1":                 {"digest"},
	"digest/sha2":                 {"digest"},
	"forwardable":                 nil,
	"getoptlong":                  nil,
	"ipaddr":                      nil,
	"json":                        nil,
	"logger":                      nil,
	"marshal":                     nil,
	"matrix":                      nil,
	"net/http":                    {"net/protocol"},
	"net/https":                   {"net/http", "net/protocol"},
	"net/protocol":                nil,
	"open-uri":                    {"uri", "net/http", "net/protocol"},
	"optparse":                    nil,
	"pathname":                    nil,
	"prime":                       nil,
	"rbconfig":                    nil,
	"resolv":                      nil,
	"random/formatter":            {"securerandom"},
	"securerandom":                {"random/formatter"},
	"shellwords":                  nil,
	"singleton":                   nil,
	"stringio":                    nil,
	"strscan":                     nil,
	"time":                        nil,
	"tsort":                       nil,
	"unicode_normalize":           {"unicode_normalize/normalize"},
	"unicode_normalize/normalize": nil,
	"uri":                         nil,
	"yaml":                        {"psych"},
	"zlib":                        nil,
	"psych":                       {"yaml"},
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
	nethttp.Install()
	pathname.Install()
	prime.Install()
	rbconfig.Install()
	securerandom.Install()
	shellwords.Install()
	singleton.Install()
	stringio.Install()
	strscan.Install()
	optparse.Install()
	getoptlong.Install()
	matrix.Install()
	bigdecimal.Install()
	resolv.Install()
	libtime.Install()
	tsort.Install()
	unicodenormalize.Install()
	uri.Install()
	// open-uri defines methods on URI, so it follows uri.
	openuri.Install()
	yaml.Install()
	zlib.Install()
}
