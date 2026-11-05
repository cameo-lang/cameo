package tests

import (
	"github.com/cameo-lang/cameo/src/lib"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// TestNativeBindsEveryGoClassUnderCameoNative proves a class or module
// the Go binary binds at the top level, of the core or of a library, is
// Cameo::Native::Name as well as Name and keeps Name as its own; that a
// top-level constant which is no class is not there, nor Cameo itself,
// whose mirror would hold it; and that a class a library written in
// cameo defines at the top level, after the binary bound its names, is
// not there either.
func TestNativeBindsEveryGoClassUnderCameoNative(t *testing.T) {
	lib.Sources["pure"] = "module Pure\nend\n"
	defer delete(lib.Sources, "pure")
	out := runScript(t, `
p Cameo::Native::JSON == JSON, Cameo::Native::String == String, Cameo::Native::Integer == Integer
p Cameo::Native::JSON::ParserError, Cameo::Native::Benchmark::Tms, Cameo::Native::Math::PI
p Cameo::Native::Pathname == Pathname, Cameo::Native::CGI == CGI, Cameo::Native::Kernel == Kernel
p Cameo::Native.class, Cameo::Native.constants.include?(:STDOUT), Cameo::Native.constants.include?(:RUBY_VERSION), Cameo::Native.constants.include?(:Cameo)
require "pure"
p Pure.class, Cameo::Native.constants.include?(:Pure)
begin
  Cameo::Native::Nope
rescue NameError => e
  puts e.message
end
`)
	assert.Equals(t, out, "true\ntrue\ntrue\nJSON::ParserError\nBenchmark::Tms\n3.141592653589793\ntrue\ntrue\ntrue\nModule\nfalse\nfalse\nfalse\nModule\nfalse\nuninitialized constant Cameo::Native::Nope\n")
}
