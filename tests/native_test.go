package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// TestNativeBindsEveryGoClassUnderCameoNative proves a class the Go
// binary defines, of the core or of a library, is Cameo::Native::Name as
// well as Name, and keeps Name as its own; a library written in cameo is
// not native and is not there.
func TestNativeBindsEveryGoClassUnderCameoNative(t *testing.T) {
	out := runScript(t, `
p Cameo::Native::JSON == JSON, Cameo::Native::String == String, Cameo::Native::Integer == Integer
p Cameo::Native::JSON::ParserError, Cameo::Native::CSV::Row, Cameo::Native::Math::PI
p Cameo::Native::TSort == TSort, Cameo::Native::HTTPD == HTTPD, Cameo::Native::Kernel == Kernel
p Cameo::Native.class
begin
  Cameo::Native::Frankie
rescue NameError => e
  puts e.message
end
`)
	assert.Equals(t, out, "true\ntrue\ntrue\nJSON::ParserError\nCSV::Row\n3.141592653589793\ntrue\ntrue\ntrue\nModule\nuninitialized constant Cameo::Native::Frankie\n")
}
