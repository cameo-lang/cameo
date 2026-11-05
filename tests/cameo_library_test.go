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

func TestALibraryWrittenInCameoThatDoesNotParseIsASyntaxError(t *testing.T) {
	lib.Sources["broken"] = "def oops(\n"
	defer delete(lib.Sources, "broken")
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `require "broken"`, &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "broken.cameo:")
	assert.Contains(t, err.Error(), "(SyntaxError)")
}

func TestALibraryWrittenInCameoThatRaisesIsNotLoaded(t *testing.T) {
	// The error names the library's file and line; the library is not
	// marked loaded, so a later require runs it again, as Ruby's does.
	lib.Sources["boom"] = "module Boom\nend\nraise ArgumentError, \"no\"\n"
	defer delete(lib.Sources, "boom")
	out := runScript(t, `
2.times do
  require "boom"
rescue LoadError => e
  puts e.message
end
`)
	assert.Equals(t, out, "cannot load boom: boom.cameo:3: no (ArgumentError)\ncannot load boom: boom.cameo:3: no (ArgumentError)\n")
}

func TestAWorkerCannotLoadALibraryWrittenInCameo(t *testing.T) {
	// A worker's top level is a snapshot every worker shares, so a
	// library's classes would land where the script never sees them; a
	// library the script loaded before is loaded on the worker too.
	lib.Sources["greeting"] = "module Greeting\n  def self.hello = \"hello\"\nend\n"
	defer delete(lib.Sources, "greeting")
	out := runScript(t, `
include Cameo
class W
  def load = require("greeting")
  def again = [require("greeting"), Greeting.hello]
end
begin
  await async(W.new, :load)
rescue LoadError => e
  puts e.message
end
require "greeting"
p await async(W.new, :again)
`)
	assert.Equals(t, out, "cannot load greeting from an async call: a library written in cameo is required at the top level\n[false, \"hello\"]\n")
}

func TestALibraryWrittenInCameoIsMarkedLoadedBeforeItRuns(t *testing.T) {
	// A library that requires itself, or two that require each other, get
	// false from the inner require; a library whose top level raises
	// leaves its name unmarked, so the next require raises again.
	lib.Sources["selfish"] = "p require(\"selfish\")\nmodule Selfish\n  def self.hello = \"hello\"\nend\n"
	lib.Sources["broken"] = "raise \"not today\"\n"
	defer delete(lib.Sources, "selfish")
	defer delete(lib.Sources, "broken")
	out := runScript(t, `
p require("selfish")
p require("selfish")
puts Selfish.hello
2.times do
  begin
    require "broken"
  rescue LoadError => e
    puts e.message
  end
end
`)
	assert.Equals(t, out, "false\ntrue\nfalse\nhello\ncannot load broken: broken.cameo:1: not today (RuntimeError)\ncannot load broken: broken.cameo:1: not today (RuntimeError)\n")
}

// An error a library's method raises at run time is reported with the
// library's file and line, and one a script's block raises inside the
// library's method with the script's: the innermost method or block an
// exception leaves names the file.
func TestAnErrorInALibraryWrittenInCameoNamesItsFile(t *testing.T) {
	lib.Sources["lib"] = "module Lib\n  def self.go\n    raise ArgumentError, \"no\"\n  end\n\n  def self.each\n    yield 1\n  end\nend\n"
	defer delete(lib.Sources, "lib")
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "require \"lib\"\nLib.go\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "lib.cameo:3: no (ArgumentError)")
	err = cameo.Run("t.cameo", "require \"lib\"\nLib.each { |x| raise \"mine\" }\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:2: mine (RuntimeError)")
}
