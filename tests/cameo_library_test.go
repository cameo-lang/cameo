package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/lib"
)

// A library written in cameo registers its source in lib.Sources; these
// tests register one of their own.

func TestRequireRunsALibraryWrittenInCameoOnce(t *testing.T) {
	lib.Sources["greeting"] = `
module Greeting
  class Hello
    string :name
    def to_s = "hello #{name}"
  end
  def self.hello(name) = Hello.new(name: name)
end
GREETED = "once"
`
	defer delete(lib.Sources, "greeting")
	out := runScript(t, `
p require("greeting")
puts Greeting.hello("frankie")
p require("greeting"), GREETED
def later = Greeting::Hello.new(name: "later").to_s
puts later
`)
	assert.Equals(t, out, "true\nhello frankie\nfalse\n\"once\"\nhello later\n")
}

func TestALibraryWrittenInCameoThatDoesNotParseIsALoadError(t *testing.T) {
	lib.Sources["broken"] = "def oops(\n"
	defer delete(lib.Sources, "broken")
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `require "broken"`, &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "cannot load broken:")
	assert.Contains(t, err.Error(), "(LoadError)")
}
