package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/object"
)

// routes are the classes the writer and the reader of these tests both
// define; the reader has none of the writer's other code.
const routes = `class Route
  string :path
  proc :handler
end
class Sub < Route
  proc :extra
end
class App
  list :routes, of: Route
  map :named, key: :string, value: Route
end
`

// field returns the field name of the instance v.
func field(t *testing.T, v object.Value, name string) object.Value {
	t.Helper()
	inst, ok := v.AsInstance()
	assert.True(t, ok, "not an instance")
	for i, f := range inst.Cls.Fields {
		if f.Name == name {
			return inst.Fields[i]
		}
	}
	t.Fatalf("%s has no field %s", inst.Cls.Name, name)
	return object.Value{}
}

// A program that has the classes and not the code reads an instance with
// procs in a map: the closures are written and read in one order, so the
// one that carries the code is read first, whatever order protobuf gives
// the entries in.
func TestAnotherProgramReadsTheProcsOfAMapField(t *testing.T) {
	writer, _ := ran(t, routes+`
add = ->(n) { ->(x) { x + n } }
names = ["m", "c", "x", "a", "k", "e"]
app = App.new(named: names.each_with_index.to_h { |name, i| [name, Route.new(path: name, handler: add.(i))] })
`)
	for round := 0; round < 20; round++ {
		data, err := writer.ProcEncoder(true).Encode(local(t, writer, "app"))
		assert.NoError(t, err)
		reader, _ := ran(t, routes)
		v, err := reader.Decoder().Decode(data)
		assert.NoError(t, err)
		named := field(t, v, "named").Map()
		got, ok := named.Get(object.String("k"))
		assert.True(t, ok, "the map has no route k")
		assert.Equals(t, called(t, reader, field(t, got, "handler"), object.Int(10)), "14")
	}
}

// An instance of a subclass in a field of its parent is written as the
// parent's message, without the procs the parent does not declare: a
// closure nobody reads would be the one that carries the code.
func TestAnotherProgramReadsASubclassWithAProcOfItsOwn(t *testing.T) {
	writer, _ := ran(t, routes+`
add = ->(n) { ->(x) { x + n } }
app = App.new(routes: [Sub.new(path: "/s", extra: add.(100)), Route.new(path: "/r", handler: add.(1))])
sub = Sub.new(path: "/s", handler: add.(2), extra: add.(3))
`)
	data, err := writer.ProcEncoder(true).Encode(local(t, writer, "app"))
	assert.NoError(t, err)
	reader, _ := ran(t, routes)
	v, err := reader.Decoder().Decode(data)
	assert.NoError(t, err)
	list := field(t, v, "routes").List().Elems
	assert.Equals(t, len(list), 2)
	assert.True(t, field(t, list[0], "handler").IsNil() || field(t, list[0], "handler").IsZero(), "the subclass's handler is set")
	assert.Equals(t, called(t, reader, field(t, list[1], "handler"), object.Int(10)), "11")

	// On its own the subclass travels as itself, with both procs.
	data, err = writer.ProcEncoder(true).Encode(local(t, writer, "sub"))
	assert.NoError(t, err)
	reader, _ = ran(t, routes)
	v, err = reader.Decoder().Decode(data)
	assert.NoError(t, err)
	assert.Equals(t, v.Instance().Cls.Name, "Sub")
	assert.Equals(t, called(t, reader, field(t, v, "handler"), object.Int(10)), "12")
	assert.Equals(t, called(t, reader, field(t, v, "extra"), object.Int(10)), "13")
}
