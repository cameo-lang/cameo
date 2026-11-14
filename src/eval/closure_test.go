package eval

import (
	"bytes"
	"github.com/cameo-lang/cameo/src/str"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// ran runs src in an interpreter of its own and returns the interpreter
// and what the script printed.
func ran(t *testing.T, src string) (*Interp, *bytes.Buffer) {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	var out bytes.Buffer
	in := New(&out, &out)
	assert.NoError(t, in.Run("t.cameo", prog))
	return in, &out
}

// local returns the top-level variable name of in.
func local(t *testing.T, in *Interp, name string) object.Value {
	t.Helper()
	v, ok := in.top.Get(name)
	assert.True(t, ok, "no top-level variable %s", name)
	return v
}

// called calls the proc v in in with args and returns the inspect of its
// result.
func called(t *testing.T, in *Interp, v object.Value, args ...object.Value) string {
	t.Helper()
	result, err := in.callBlock(v, args, nil, object.Value{})
	assert.NoError(t, err)
	s, err := in.Inspect(result)
	assert.NoError(t, err)
	return s
}

// A proc that read locals when it was made travels with their values, and
// runs in a program that never saw its source.
func TestAProcTravelsWithTheValuesItCaptured(t *testing.T) {
	writer, _ := ran(t, `factor = 3
names = ["a", "b"]
unused = Random.new(1)
scale = ->(x) { names.map { |n| n * (x * factor) } }
`)
	data, err := writer.ProcEncoder(true).Encode(local(t, writer, "scale"))
	assert.NoError(t, err)

	reader, _ := ran(t, "")
	v, err := reader.Decoder().Decode(data)
	assert.NoError(t, err)
	assert.Equals(t, called(t, reader, v, object.Int(1)), `["aaa", "bbb"]`)
	p := proc(v)
	assert.True(t, p.Strict, "a lambda reads back as a lambda")

	// The message carries the unit once, and of the locals in sight only
	// the two the code reads: unused is a Random, which cannot travel,
	// and does not stop the proc.
	msg := &cameopb.Value{}
	assert.NoError(t, proto.Unmarshal(data, msg))
	closure := msg.GetClosure()
	assert.Equals(t, closure.GetCode().GetDigest(), closure.GetUnit())
	var locals []string
	for _, l := range closure.GetBinding().GetLocals() {
		locals = append(locals, l.GetName())
	}
	assert.Equals(t, strings.Join(locals, ","), "factor,names")
}

func TestALambdaThatCallsItselfTravels(t *testing.T) {
	writer, _ := ran(t, "fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }\n")
	data, err := writer.ProcEncoder(true).Encode(local(t, writer, "fact"))
	assert.NoError(t, err)
	reader, _ := ran(t, "")
	v, err := reader.Decoder().Decode(data)
	assert.NoError(t, err)
	assert.Equals(t, called(t, reader, v, object.Int(5)), "120")
}

// A proc written in a method travels with its self, by value, and with
// the method it was written in, by name: the reader has the class, and
// the proc's super and its calls on self go to it.
func TestAProcTravelsWithItsSelfAndItsMethod(t *testing.T) {
	classes := `class Base
  def greet(name) = "hello #{name}"
end
class Greeter < Base
  string :punct
  def initialize(punct) = self.punct = punct
  def greet(name) = proc { |twice| (super + punct) * (twice ? 2 : 1) }
  def self.maker = ->(x) { new(x) }
  def each_twice = proc { |x| yield(x) + yield(x) }
end
`
	writer, _ := ran(t, classes+`greet = Greeter.new("!").greet("ann")
maker = Greeter.maker
twice = Greeter.new("?").each_twice { |x| x * 10 }
`)
	reader, _ := ran(t, classes)
	travel := func(name string) object.Value {
		data, err := writer.ProcEncoder(true).Encode(local(t, writer, name))
		assert.NoError(t, err)
		v, err := reader.Decoder().Decode(data)
		assert.NoError(t, err)
		return v
	}
	assert.Equals(t, called(t, reader, travel("greet"), object.False), `"hello ann!"`)
	assert.Equals(t, called(t, reader, travel("greet"), object.True), `"hello ann!hello ann!"`)
	assert.Equals(t, called(t, reader, travel("maker"), object.NewString(".", str.UTF8)), `Greeter(punct: ".")`)
	assert.Equals(t, called(t, reader, travel("twice"), object.Int(2)), "40")
}

func TestProcsOfSymbolsAndHashesTravel(t *testing.T) {
	writer, out := ran(t, `up = :upcase.to_proc
table = {"a" => 1}.to_proc
class Tag
  string :text
end
klass = Tag
make = -> { klass.new(text: "made").text }
puts from_value_proto(up.to_value_proto).call("x")
puts from_value_proto(table.to_value_proto).call("a")
puts from_value_proto(make.to_value_proto).call
puts from_value_proto([up, make].to_value_proto).map { |f| f.lambda? }.inspect
`)
	assert.Equals(t, out.String(), "X\n1\nmade\n[false, true]\n")
	// A reader in the same program has the unit, and needs none in the
	// message.
	data, err := writer.ProcEncoder(false).Encode(local(t, writer, "make"))
	assert.NoError(t, err)
	msg := &cameopb.Value{}
	assert.NoError(t, proto.Unmarshal(data, msg))
	assert.True(t, msg.GetClosure().GetCode() == nil, "the closure carries its unit")
	v, err := writer.Decoder().Decode(data)
	assert.NoError(t, err)
	assert.Equals(t, called(t, writer, v), `"made"`)
	// A reader that is another program does.
	reader, _ := ran(t, "")
	_, err = reader.Decoder().Decode(data)
	assert.True(t, err != nil && strings.Contains(err.Error(), "neither in the message nor in this program"), "decoding without the unit: %v", err)
}

func TestAProcSaysWhatKeepsItFromTravelling(t *testing.T) {
	writer, _ := ran(t, `dice = Random.new(1)
reads = -> { dice.rand }
builtin = 5.method(:+).to_proc rescue nil
`)
	_, err := writer.ProcEncoder(true).Encode(local(t, writer, "reads"))
	assert.True(t, err != nil, "a proc that reads a Random was encoded")
	assert.Equals(t, err.Error(), "a Proc that captured dice, a Random cannot be serialised")

	_, err = writer.Encoder().Encode(local(t, writer, "reads"))
	assert.True(t, err != nil, "the encoder without procs encoded one")
	assert.Equals(t, err.Error(), "a Proc cannot be serialised")

	_, err = writer.ProcEncoder(true).Encode(writer.Block(func([]object.Value) (object.Value, error) { return object.Nil, nil }))
	assert.True(t, err != nil, "a proc a builtin made was encoded")
	assert.Equals(t, err.Error(), "a Proc a builtin made cannot be serialised")
}

// A proc is an argument of an async call and its result like any other
// value: the worker gets the proc with what it captured, and what a worker
// made is called by the script that awaits it.
func TestProcsCrossToAWorkerAndBack(t *testing.T) {
	_, out := ran(t, `include Cameo
class Work
  def apply(f, xs) = xs.map { |x| f.(x) }
  def adder(n) = ->(x) { x + n }
  def twice(&blk) = blk
  def compose(f, g:) = ->(x) { g.(f.(x)) }
end
w = Work.new
factor = 3
scale = ->(x) { x * factor }
puts await(w, :apply, scale, [1, 2]).inspect
add = await(w, :adder, 10)
puts add.(5), add.lambda?
both = await(w, :compose, scale, g: add)
puts both.(2)
pr = promise
resolve(pr, ->(x) { x.to_s * factor })
puts await(pr).(7)
puts await([async(w, :adder, 1), async(w, :adder, 2)]).map { |f| f.(0) }.inspect
`)
	assert.Equals(t, out.String(), "[3, 6]\n15\ntrue\n16\n777\n[1, 2]\n")
}

// A Hash with a default proc travels with the proc, and computes its
// defaults where it arrives.
func TestAHashTravelsWithItsDefaultProc(t *testing.T) {
	_, out := ran(t, `include Cameo
class Work
  def lookup(table, key) = table[key]
  def table = Hash.new { |h, k| "made #{k}" }.merge({"a" => "had a"})
end
step = 10
squares = Hash.new { |h, k| k * k + step }.merge({1 => "one"})
copy = from_value_proto(squares.to_value_proto)
p copy[1], copy[4], copy.size, copy.default_proc.lambda?
w = Work.new
p await(w, :lookup, squares, 3)
made = await(w, :table)
p made["a"], made["z"]
`)
	assert.Equals(t, out.String(), "\"one\"\n26\n1\nfalse\n19\n\"had a\"\n\"made z\"\n")
}

// A symbol a proc captured reads back as a String, as a symbol does
// wherever a value crosses the envelope (doc/design/cameo.md, "Symbols").
func TestASymbolAProcCapturedReadsBackAsAString(t *testing.T) {
	_, out := ran(t, `mode = :fast
check = -> { [mode, mode == :fast] }
p check.call, from_value_proto(check.to_value_proto).call
`)
	assert.Equals(t, out.String(), "[:fast, true]\n[\"fast\", false]\n")
}

// A return in a proc that travelled has no method to return from.
func TestAReturnInAProcThatTravelledRaises(t *testing.T) {
	classes := "def make = proc { |x| return x }\n"
	writer, _ := ran(t, classes+"pr = make\n")
	data, err := writer.ProcEncoder(true).Encode(local(t, writer, "pr"))
	assert.NoError(t, err)
	reader, _ := ran(t, classes)
	v, err := reader.Decoder().Decode(data)
	assert.NoError(t, err)
	_, err = reader.callBlock(v, []object.Value{object.Int(1)}, nil, object.Value{})
	assert.True(t, err != nil, "the return returned")
	assert.Equals(t, reader.FormatError(err), "t.cameo:1: unexpected return (LocalJumpError)")
}
