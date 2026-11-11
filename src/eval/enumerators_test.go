package eval

import (
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// methodsOf lists the names of the methods of the classes and modules a
// value of class cls answers with, Object's and Kernel's left out: those
// print, read and exit.
func methodsOf(cls *object.Class) []string {
	seen := map[string]bool{}
	var walk func(c *object.Class)
	walk = func(c *object.Class) {
		if c == nil || c == object.ObjectClass || c == object.BasicObjectClass {
			return
		}
		for name := range c.Methods {
			seen[name] = true
		}
		for _, m := range c.Includes {
			walk(m)
		}
		walk(c.Super)
	}
	walk(cls)
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// first lists up to n elements of the enumerator v, as first(n) does.
func first(t *testing.T, in *Interp, v object.Value, n int64) string {
	t.Helper()
	l, err := in.CallWith(v, "first", []object.Value{object.Int(n)}, nil, object.Value{})
	if err != nil {
		return "error: " + err.Error()
	}
	s, err := in.Inspect(l)
	assert.NoError(t, err)
	return s
}

// Every method of the core that returns an enumerator when it is called
// without a block returns one that travels: the method is among
// object.MakesEnumerator's, and the call makes an enumerator with the
// same inspect and the same first elements. The test calls every method
// of the core's values with no argument, one and two, so a method added
// later that makes an enumerator fails here until it says how it is made
// again.
func TestEveryEnumeratorOfTheCoreTravels(t *testing.T) {
	in, _ := ran(t, `list = [3, 1, 2]
hash = {"a" => 1, "b" => 2}
set = Set.new([1, 2])
range = (1..4)
int = 3
float = 1.5
string = "ab\ncd"
eager = [1, 2, 3].each
lazy = [1, 2, 3].lazy
sliced = [1, 2, 3].each_slice(2)
`)
	argSets := [][]object.Value{nil, {object.Int(2)}, {object.Int(1), object.Int(2)}}
	var missing []string
	made := 0
	for _, sample := range []string{"list", "hash", "set", "range", "int", "float", "string", "eager", "lazy", "sliced"} {
		recv := local(t, in, sample)
		for _, name := range append(methodsOf(recv.Class()), "then", "yield_self", "to_enum", "enum_for") {
			for _, args := range argSets {
				v, err := in.CallWith(recv, name, args, nil, object.Value{})
				if err != nil || v.Kind() != object.KindEnumerator {
					continue
				}
				call := sample + "." + name
				if len(args) > 0 {
					call += "/" + string(rune('0'+len(args)))
				}
				e := v.Enumerator()
				if e.Make == "" && e.Of == nil {
					missing = append(missing, call+" (made by no call)")
					continue
				}
				data, err := in.ProcEncoder(false).Encode(v)
				if err != nil {
					missing = append(missing, call+" ("+err.Error()+")")
					continue
				}
				back, err := in.Decoder().Decode(data)
				if err != nil {
					missing = append(missing, call+" ("+err.Error()+")")
					continue
				}
				want, err := in.Inspect(v)
				assert.NoError(t, err)
				got, err := in.Inspect(back)
				assert.NoError(t, err)
				if got != want || back.Class() != v.Class() || first(t, in, back, 5) != first(t, in, v, 5) {
					missing = append(missing, call+" (reads back as "+got+" with "+first(t, in, back, 5)+", was "+want+" with "+first(t, in, v, 5)+")")
					continue
				}
				made++
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d enumerators do not travel:\n%s", len(missing), strings.Join(missing, "\n"))
	}
	// The walk found the enumerators: a suite that made none passes too.
	assert.True(t, made > 100, "the walk made %d enumerators, want more than 100", made)
}

// Reading a message makes no call but one that makes an enumerator and
// does nothing else: not a method that computes, not a script's method,
// and no block where the method would run it.
func TestAMessageOfAnEnumeratorRunsNothing(t *testing.T) {
	in, out := ran(t, `class Bag
  include Enumerable
  int :n
  def each = puts("each ran")
  def each_slice(k) = puts("each_slice ran")
end
bag = Bag.new(n: 1)
noisy = -> (x) { puts "the block ran" }
list = [1, 2]
hash = {"a" => "b"}
`)
	enc := in.ProcEncoder(false)
	value := func(name string) *cameopb.Value {
		m, err := enc.Message(local(t, in, name))
		assert.NoError(t, err)
		return m
	}
	refused := func(what string, e *cameopb.Enumerator, want string) {
		t.Helper()
		data, err := proto.Marshal(&cameopb.Value{Kind: &cameopb.Value_Enumerator{Enumerator: e}})
		assert.NoError(t, err)
		_, err = in.Decoder().Decode(data)
		assert.True(t, err != nil, "%s was read", what)
		assert.Contains(t, err.Error(), want)
	}
	refused("a method that computes", &cameopb.Enumerator{Recv: value("list"), Method: "sum"}, "not known to make one")
	refused("send", &cameopb.Enumerator{Recv: value("list"), Method: "send", Args: []*cameopb.Value{{Kind: &cameopb.Value_StringValue{StringValue: "exit"}}}}, "not known to make one")
	refused("a search", &cameopb.Enumerator{Recv: value("list"), Method: "index", Args: []*cameopb.Value{value("bag")}}, "not known to make one")
	refused("a block on an Array", &cameopb.Enumerator{Recv: value("list"), Method: "map", Block: value("noisy").GetClosure()}, "not known to make one")
	refused("lazy of a script's object", &cameopb.Enumerator{Recv: value("bag"), Method: "lazy"}, "not known to make one")
	refused("a script's method", &cameopb.Enumerator{Recv: value("bag"), Method: "each_slice", Args: []*cameopb.Value{{Kind: &cameopb.Value_IntValue{IntValue: 2}}}}, "has no builtin method each_slice")
	refused("a method that makes none here", &cameopb.Enumerator{Recv: value("hash"), Method: "transform_keys", Args: []*cameopb.Value{value("hash")}}, "transform_keys made no Enumerator")
	assert.Equals(t, out.String(), "")

	// to_enum of a script's object is made without calling its method;
	// the method runs when the enumerator does.
	data, err := proto.Marshal(&cameopb.Value{Kind: &cameopb.Value_Enumerator{Enumerator: &cameopb.Enumerator{Recv: value("bag"), Method: "each", ToEnum: true}}})
	assert.NoError(t, err)
	v, err := in.Decoder().Decode(data)
	assert.NoError(t, err)
	assert.Equals(t, out.String(), "")
	s, err := in.Inspect(v)
	assert.NoError(t, err)
	assert.Equals(t, s, "#<Enumerator: Bag(n: 1):each>")

	// A reader that makes no enumerators says so.
	_, err = protogen.NewDecoder(in.Proto(), in.FindClass).Decode(data)
	assert.True(t, err != nil, "a decoder without a maker read an Enumerator")
	assert.Contains(t, err.Error(), "which this reader does not decode")
}
