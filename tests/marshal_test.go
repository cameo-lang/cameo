package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

func TestMarshalRoundTripsValuesInTheEnvelope(t *testing.T) {
	out := runScript(t, `require "marshal"
class Pt
  int :x
  string :label, default: "p"
  def initialize(x, label: "p")
    self.x = x
    self.label = label
  end
end
class Boom < StandardError
  int :code
end
[nil, true, 42, -2**70, 1.5, "str", "\xff".b, [1, [2, "x"]], {"b" => [nil]}, Set[1, 2], 1..3, 1..., Time.utc(2020, 1, 2, 3, 4, 5), /a.b/i].each do |v|
  d = Marshal.dump(v)
  p [d.encoding, Marshal.load(d) == v, Marshal.load(d)]
end
pt = Marshal.load(Marshal.dump(Pt.new(3, label: "q")))
p pt, pt.class, pt == Pt.new(3, label: "q"), Marshal.load(Marshal.dump([Pt.new(1), {"k" => Pt.new(2)}]))
e = Marshal.load(Marshal.dump(Boom.new("bad", code: 7)))
p e.class, e.message, e.code, Marshal.load(Marshal.dump(:sym)), Marshal.load(Marshal.dump({a: 1})), Marshal.dump(1).bytes, Marshal.restore(Marshal.dump("r")), Marshal.load(Marshal.dump("a"), freeze: true)
[-> { 1 }, String, Random.new(1), Marshal].each do |v|
  Marshal.dump(v)
rescue TypeError => e
  puts e.message
end
# protobuf-go varies the spacing of its messages from build to build, so
# only the start of the message is checked.
[nil, 1, "garbage"].each do |v|
  Marshal.load(v)
rescue TypeError, ArgumentError => e
  puts "#{e.class}: #{e.message.sub(/proto:.*/, "proto")}"
end
begin; Marshal.dump("a", "io"); rescue TypeError => e; puts e.message; end
`)
	assert.Equals(t, out, `[#<Encoding:BINARY (ASCII-8BIT)>, true, nil]
[#<Encoding:BINARY (ASCII-8BIT)>, true, true]
[#<Encoding:BINARY (ASCII-8BIT)>, true, 42]
[#<Encoding:BINARY (ASCII-8BIT)>, true, -1180591620717411303424]
[#<Encoding:BINARY (ASCII-8BIT)>, true, 1.5]
[#<Encoding:BINARY (ASCII-8BIT)>, true, "str"]
[#<Encoding:BINARY (ASCII-8BIT)>, true, "\xFF"]
[#<Encoding:BINARY (ASCII-8BIT)>, true, [1, [2, "x"]]]
[#<Encoding:BINARY (ASCII-8BIT)>, true, {"b" => [nil]}]
[#<Encoding:BINARY (ASCII-8BIT)>, true, #<Set: {1, 2}>]
[#<Encoding:BINARY (ASCII-8BIT)>, true, 1..3]
[#<Encoding:BINARY (ASCII-8BIT)>, true, 1...]
[#<Encoding:BINARY (ASCII-8BIT)>, true, 2020-01-02 03:04:05 UTC]
[#<Encoding:BINARY (ASCII-8BIT)>, true, /a.b/i]
Pt(x: 3, label: "q")
Pt
true
[Pt(x: 1, label: "p"), {"k" => Pt(x: 2, label: "p")}]
Boom
"bad"
7
"sym"
{"a" => 1}
[16, 1]
"r"
"a"
no _dump_data is defined for class Proc
no _dump_data is defined for class Class
no _dump_data is defined for class Random
no _dump_data is defined for class Module
TypeError: instance of IO needed
TypeError: instance of IO needed
ArgumentError: incompatible marshal data: proto
Marshal.dump writes a String: an IO is not supported
`)
}
