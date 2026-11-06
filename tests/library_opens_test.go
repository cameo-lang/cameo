package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/lib"
)

// opener is a library, written in cameo, that adds to JSON, a module Go
// defines: a constant and a class method by reopening it, a class by
// defining it inside.
const opener = `module JSON
  ADDED = "by a library"
  def self.greet = "hi from #{name}"
end

class JSON::Thing
  int :x
  def twice = x * 2
  # JSON in a method of a class inside the module is the program's copy,
  # with what the library added, not the module Go defined.
  def greeting = JSON.greet
end
`

// TestALibraryOpensAModuleGoDefined proves a library written in cameo may
// add to a module Go defined: the program sees the additions with what Go
// defined, JSON.generate and JSON::ParserError among it; another program
// in the process sees none of them; and a script still cannot reopen the
// module, nor a library a class Go defined.
func TestALibraryOpensAModuleGoDefined(t *testing.T) {
	lib.Sources["test/opener"] = opener
	lib.Sources["test/string"] = "class String\n  def shout = upcase\nend\n"
	defer delete(lib.Sources, "test/opener")
	defer delete(lib.Sources, "test/string")

	out := runScript(t, `require "json"
require "test/opener"
p JSON::ADDED, JSON.greet, JSON::Thing.new(x: 21).twice, JSON::Thing.name, JSON::Thing.new.greeting
p JSON.respond_to?(:generate), JSON.generate([1]), JSON::ParserError.ancestors.include?(StandardError)
begin
  raise JSON::ParserError, "same class"
rescue JSON::ParserError => e
  p e.message
end
`)
	assert.Equals(t, out, `"by a library"
"hi from JSON"
42
"JSON::Thing"
"hi from JSON"
true
"[1]"
true
"same class"
`)
	// Another program, which did not require the library, sees JSON as
	// Go defined it.
	out = runScript(t, `require "json"
p JSON.const_defined?(:ADDED) rescue p JSON.constants.include?(:ADDED)
p JSON.respond_to?(:greet)
`)
	assert.Equals(t, out, "false\nfalse\n")

	for src, want := range map[string]string{
		"module JSON\n  X = 1\nend\n": "t.cameo:1: cannot redefine the builtin module JSON (CameoError)",
		"class JSON::Mine\nend\n":     "t.cameo:1: cannot define anything inside JSON, which the script did not define (CameoError)",
		"require \"test/string\"\n":   "t.cameo:1: cannot load test/string: test/string.cameo:1: cannot redefine the builtin class String (CameoError) (LoadError)",
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.Run("t.cameo", src, &stdout, &stderr)
		assert.NotNil(t, err, src)
		assert.Equals(t, err.Error(), want, src)
	}
}
