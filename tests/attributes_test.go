package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// Class attributes are cameo's own; these outputs are their
// specification. A class is built in its body as an instance is in
// initialize: self.name = v sets an attribute while the body runs, from
// the body or from a class method it calls, and the class is sealed when
// the body ends.

func TestAClassBodySetsAttributesAndIsSealedAfter(t *testing.T) {
	out := runScript(t, `
class Switches
  attribute :switches, default: []
  attribute :label

  def self.on(name, &step)
    self.switches += [[name, step]]
  end
  def self.names = switches.map { |n, _| n }
end
class Options < Switches
  bool :verbose, default: false
  string :name, default: "world"
  on("--verbose") { |o, _| o.with(verbose: true) }
  on("--name") { |o, arg| o.with(name: arg.to_s) }
  self.label = "opts"
end
p Switches.names, Options.names, Switches.label, Options.label
o = Options.switches.reduce(Options.new) { |o, (_, step)| step.call(o, nil) }
p o
begin
  Options.on("--late") { }
rescue CameoError => e
  puts e.message
end
begin
  Options.new.class.on("--late") { }
rescue CameoError => e
  puts e.message
end
`)
	assert.Equals(t, out, `[]
["--verbose", "--name"]
nil
"opts"
Options(verbose: true, name: "")
self.switches = …: Options is sealed: an attribute is set while the class body runs
self.switches = …: Options is sealed: an attribute is set while the class body runs
`)
}

func TestReopeningAndClassNewSetAttributesToo(t *testing.T) {
	out := runScript(t, `
class Base
  attribute :names, default: []
  def self.add(n) = self.names += [n]
end
class Options < Base
  add(:a)
end
Before = Options
class Options
  attribute :names, default: []
  add(:b)
end
p Before.names, Options.names, Base.names
C = Class.new(Base) { add(:c) }
p C.names
D = Class.new(Base) do |me|
  me.names.size
end
p D.names
`)
	assert.Equals(t, out, "[:a]\n[:a, :b]\n[]\n[:c]\n[]\n")
}

func TestAttributeErrors(t *testing.T) {
	out := runScript(t, `
class A
  attribute :x
  def self.set = self.y = 1
  begin
    set
  rescue NameError => e
    puts e.message
  end
  begin
    attribute :x
  rescue CameoError => e
    puts e.message
  end
  def self.x2 = 1
  begin
    attribute :x2
  rescue CameoError => e
    puts e.message
  end
end
p A.x
`)
	assert.Equals(t, out, `self.y = …: A has no attribute y
attribute x is already declared in A
attribute x2: A already defines a class method x2
nil
`)
	for _, c := range []struct{ src, want string }{
		{"class A\n  attribute :x\n  def m\n    self.x = 1\n  end\nend\n", "self.x = …: a field is set in initialize, and a class attribute in a class body or a class method; elsewhere use with"},
		{"class A\n  attribute :x, :y, default: 1, other: 2\nend\n", "attribute: the one option is default:"},
		{"class A\n  attribute \"x\"\nend\n", "attribute: an attribute is named by a symbol"},
		{"module M\n  attribute :x\nend\n", "attribute :x: an attribute belongs to a class body"},
	} {
		var stdout, stderr bytes.Buffer
		err := cameo.Run("t.cameo", c.src, &stdout, &stderr)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), c.want)
	}
}
