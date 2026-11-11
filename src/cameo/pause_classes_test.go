package cameo

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
)

// resumed runs src to its pause, dumps it, and runs the dump in a program
// of its own, and that one to its pause, until the script ends. It
// returns what the script printed before the first pause, what it printed
// after, and the first dump.
func resumed(t *testing.T, src string) (before, after string, dump *Dump) {
	t.Helper()
	prog, out := pausing(t, src)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	var rest bytes.Buffer
	for {
		d, err := prog.Dump()
		if err != nil {
			t.Fatal(err)
		}
		if dump == nil {
			dump = d
		}
		cont, err := Resume(d.Data, &rest, &rest)
		if err != nil {
			t.Fatal(err)
		}
		cont.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) {
			cont.Pause()
			return nil, nil
		})
		prog = cont
		if err := cont.Run(); !errors.Is(err, ErrPaused) {
			if err != nil {
				t.Fatal(err)
			}
			return out.String(), rest.String(), dump
		}
	}
}

// whole runs src without its pauses and returns what it printed.
func whole(t *testing.T, src string) string {
	t.Helper()
	prog, out := pausing(t, strings.ReplaceAll(src, "Test.pause\n", ""))
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// sameAsWhole proves src prints the same paused and resumed as in one
// run, and returns the dump.
func sameAsWhole(t *testing.T, src string) *Dump {
	t.Helper()
	before, after, dump := resumed(t, src)
	if got, want := before+after, whole(t, src); got != want {
		t.Fatalf("paused and resumed the script printed\n%s\nand in one run\n%s", got, want)
	}
	return dump
}

// A resumed run has the classes of the dump, and runs no definition
// again: a class body that prints does not print twice, and a method a
// condition defined is there, which no definition statement would bring
// back.
func TestResumeRunsNoDefinitionAgain(t *testing.T) {
	before, after, _ := resumed(t, `class Point
  puts "defining Point"
  int :x, :y
  ORIGIN = [0, 0]
  def dist = x + y
  def self.origin = new(x: ORIGIN[0], y: ORIGIN[1])
end
def helper(n) = n * 2
if Point.origin.dist == 0
  def late = "late"
  class Point
    def scaled(k) = Point.new(x: x * k, y: y * k)
  end
end
[1].each { |i| KEPT = -> { helper(i) } }
Test.pause
puts Point.new(x: 1, y: 2).scaled(3).dist, late, helper(4), KEPT.call
puts Point.origin.inspect, Point::ORIGIN.inspect
`)
	if before != "defining Point\n" {
		t.Fatalf("before the pause the script printed %q", before)
	}
	if want := "9\nlate\n8\n2\nPoint(x: 0, y: 0)\n[0, 0]\n"; after != want {
		t.Fatalf("after the resume the script printed %q, want %q", after, want)
	}
}

// Everything a class body declares comes back as it was: fields with
// their numbers, types and defaults, reserved numbers, attributes,
// constants, class methods, aliases, modules included and extended, and
// classes inside modules.
func TestResumeBringsBackWhatAClassDeclares(t *testing.T) {
	dump := sameAsWhole(t, `require "singleton"
module Geo
  SCALE = 10
  module Named
    def label = "#{self.class.name} #{name}"
  end
  module Counted
    def made = "made by #{name}"
  end
  class Shape
    include Named
    extend Counted
    string :name, number: 1
    reserved 2, :colour
    list :tags, of: :string, number: 3
    attribute :unit, default: "px"
    def area = 0
    alias size area
    alias shown to_s
    def to_s = "<#{label}>"
  end
  class Square < Shape
    int :side, number: 4, default: 2
    ref :inner, Shape, number: 5
    enum :mode, values: [:thin, :bold], number: 6
    attribute :unit, default: "cm"
    def area = side * side * SCALE
  end
end
class Registry
  include Singleton
  def greet = "hello"
end
class Oops < StandardError
  string :code
end
Other = Class.new(Geo::Shape) { def area = 7 }
sq = Geo::Square.new(name: "s", tags: ["a"], inner: Geo::Shape.new(name: "in"), mode: :bold)
Test.pause
puts sq.area, sq.size, sq.to_s, sq.label, sq.side, sq.inner.name, sq.mode.inspect
puts Geo::Square.unit, Geo::Shape.unit, Geo::Shape.made, Geo::Square.made
puts Geo::Square.new(name: "t").tags.inspect, Geo::Shape.new(name: "u").size
puts Other.new(name: "o").area, Other.name, Other.superclass
puts Registry.instance.greet, (Registry.new rescue "no new")
puts Geo::Square.ancestors.first(4).inspect, sq.to_proto.bytes.sort.inspect
begin
  raise Oops.new("bad", code: "E1")
rescue Oops => e
  puts e.code, e.message
end
puts (Geo::Square.new(name: "x", colour: 1) rescue "no colour")
puts sq == Geo::Square.new(name: "s", tags: ["a"], inner: Geo::Shape.new(name: "in"), mode: :bold)
`)
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
}

// A class statement on a class that is there makes a new class, and what
// was made of the class before keeps the class it had: a subclass its
// superclass, an instance its class. A resumed run has both.
func TestResumeKeepsAReopenedClassApart(t *testing.T) {
	sameAsWhole(t, `class Base
  def base = "base"
end
class Child < Base
end
module Tools
  def tool = "tool"
end
class User
  include Tools
end
class Base
  def extra = "extra"
end
module Tools
  def more = "more"
end
Test.pause
puts Base.new.extra, (Child.new.extra rescue "no extra"), Child.new.base, Child.superclass == Base
puts User.new.tool, (User.new.more rescue "no more")
class Later < Base
end
puts Later.new.extra
`)
}

// An instance made before its class was reopened keeps the class it was
// made of, which no constant leads to any more. The dump carries that
// class for it, also through a second pause.
func TestResumeKeepsAnInstanceOfAReopenedClass(t *testing.T) {
	sameAsWhole(t, `class Point
  int :x
  def double = x * 2
end
class Gone < StandardError
  string :code
end
old = Point.new(x: 2)
kept = [Gone.new("gone", code: "G1"), {"k" => old}]
class Point
  int :y
  def sum = x + y
end
class Gone
  def hint = "hint"
end
Test.pause
puts old.double, (old.sum rescue "no sum"), old.is_a?(Point), old.inspect
puts kept[0].code, (kept[0].hint rescue "no hint"), kept[1]["k"] == old
now = Point.new(x: 1, y: 2)
Test.pause
puts old.double, now.sum, now.class == old.class, old == Point.new(x: 2)
`)
}

// A library written in cameo is the script's code: its classes and
// methods are in the dump, the module Go defined that it added to
// among them, and require does not load it again.
func TestResumeBringsBackALibraryWrittenInCameo(t *testing.T) {
	dump := sameAsWhole(t, `require "shellwords"
require "abbrev"
require "tsort"
class Deps
  include TSort
  map :edges, key: :string, value: :string
  def tsort_each_node(&blk) = edges.keys.each(&blk)
  def tsort_each_child(n, &blk) = (edges[n] || "").split.each(&blk)
end
Test.pause
puts Shellwords.split("a 'b c' d").inspect, "x y".shellsplit.inspect
puts Abbrev.abbrev(["car", "cat"]).keys.sort.inspect
puts Deps.new(edges: {"a" => "b c", "b" => "c", "c" => ""}).tsort.inspect
puts require("shellwords")
`)
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(dump.Data, d); err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, unit := range d.Units {
		files[unit.GetFile()] = true
	}
	for _, file := range []string{"t.cameo", "shellwords.cameo", "tsort.cameo"} {
		if !files[file] {
			t.Fatalf("the dump has no unit of %s: %v", file, files)
		}
	}
}

// The dump names its classes for a reader, and a class that is no longer
// bound to its name by its digest.
func TestInspectDumpListsTheClasses(t *testing.T) {
	prog, _ := pausing(t, `module Geo
  class Point
  end
end
class Base
end
class Child < Base
end
class Base
  def more = 1
end
Test.pause
puts 1
`)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(dump.Data, d); err != nil {
		t.Fatal(err)
	}
	var old string
	for _, c := range d.Classes {
		if strings.HasPrefix(c.Id, "Base@") {
			old = c.Id
		}
	}
	want := "classes:\n  class Base\n  class Child\n  module Geo\n  class " + old + "\n  class Geo::Point\n"
	if old == "" || !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it:\n%s", text, want)
	}
}

// A dump resumes on a cameo of its IR version: its code is instructions
// of that version.
func TestResumeRefusesAnotherIRVersion(t *testing.T) {
	prog, _ := pausing(t, "def f = 1\nTest.pause\nputs f\n")
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(dump.Data, d); err != nil {
		t.Fatal(err)
	}
	rewritten := func(change func()) []byte {
		t.Helper()
		change()
		data, err := proto.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	version := d.Version
	_, err = Resume(rewritten(func() { d.Version = version + 1 }), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "a dump resumes on a cameo of its version") {
		t.Fatalf("resuming a dump of another version: %v", err)
	}
	// A dump from before the dump carried code has no version.
	_, err = Resume(rewritten(func() { d.Version = 0 }), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "IR version 0") {
		t.Fatalf("resuming a dump without a version: %v", err)
	}
	// Code that is not what its digest names is refused.
	_, err = Resume(rewritten(func() { d.Version = version; d.Units[0].Procs[0].Line++ }), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "names digest") {
		t.Fatalf("resuming a dump with changed code: %v", err)
	}
}
