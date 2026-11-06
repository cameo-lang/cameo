package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/lib"
)

// opener is a library, written in cameo, that adds to HTTPD, a module Go
// defines: a constant and a class method by reopening it, a class by
// defining it inside.
const opener = `module HTTPD
  ADDED = "by a library"
  def self.greet = "hi from #{name}"
end

class HTTPD::Thing
  int :x
  def twice = x * 2
  # HTTPD in a method of a class inside the module is the program's copy,
  # with what the library added, not the module Go defined.
  def greeting = HTTPD.greet
end
`

// TestALibraryOpensAModuleGoDefined proves a library written in cameo may
// add to a module Go defined, as rack.cameo would to a Rack Go defines:
// the program sees the additions with what Go defined, HTTPD.serve and
// HTTPD::Error among it; another program in the process sees none of
// them; and a script still cannot reopen the module, nor a library a class
// Go defined.
func TestALibraryOpensAModuleGoDefined(t *testing.T) {
	lib.Sources["test/opener"] = opener
	lib.Sources["test/string"] = "class String\n  def shout = upcase\nend\n"
	defer delete(lib.Sources, "test/opener")
	defer delete(lib.Sources, "test/string")

	out := runScript(t, `require "io/httpd"
require "test/opener"
p HTTPD::ADDED, HTTPD.greet, HTTPD::Thing.new(x: 21).twice, HTTPD::Thing.name, HTTPD::Thing.new.greeting
p HTTPD.respond_to?(:serve), HTTPD.allowed?, HTTPD::Error.ancestors.include?(StandardError)
begin
  raise HTTPD::Error, "same class"
rescue HTTPD::Error => e
  p e.message
end
`)
	assert.Equals(t, out, `"by a library"
"hi from HTTPD"
42
"HTTPD::Thing"
"hi from HTTPD"
true
false
true
"same class"
`)
	// Another program, which did not require the library, sees HTTPD as
	// Go defined it.
	out = runScript(t, `require "io/httpd"
p HTTPD.const_defined?(:ADDED) rescue p HTTPD.constants.include?(:ADDED)
p HTTPD.respond_to?(:greet)
`)
	assert.Equals(t, out, "false\nfalse\n")

	for src, want := range map[string]string{
		"module HTTPD\n  X = 1\nend\n": "t.cameo:1: cannot redefine the builtin module HTTPD (CameoError)",
		"class HTTPD::Mine\nend\n":     "t.cameo:1: cannot define anything inside HTTPD, which the script did not define (CameoError)",
		"require \"test/string\"\n":    "t.cameo:1: cannot load test/string: test/string.cameo:1: cannot redefine the builtin class String (CameoError) (LoadError)",
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.Run("t.cameo", src, &stdout, &stderr)
		assert.NotNil(t, err, src)
		assert.Equals(t, err.Error(), want, src)
	}
}
