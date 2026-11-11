package tests

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// proc fields are cameo's own; these outputs are their specification.

func TestProcFieldHoldsABlockAndSurvivesWith(t *testing.T) {
	out := runScript(t, `
class Route
  string :path
  proc :handler
end
r = Route.new(path: "/hi", handler: proc { |x| "hi #{x}" })
puts r.handler.call("you")
r2 = r.with(path: "/bye")
puts r2.handler.call("bye"), r2.path
puts r == r.with(path: "/hi"), r == r2
p r
`)
	assert.Equals(t, out, "hi you\nhi bye\n/bye\ntrue\nfalse\nRoute(path: \"/hi\", handler: #<Proc:t.cameo:6>)\n")
}

func TestProcFieldRefusesWhatIsNoBlockAndADefault(t *testing.T) {
	out := runScript(t, `
class Route
  proc :handler
end
begin
  Route.new(handler: "x")
rescue TypeError => e
  puts e.message
end
p Route.new.handler
`)
	assert.Equals(t, out, "field handler of Route takes a Proc, not an instance of String\nnil\n")
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "class Route\n  proc :handler, default: nil\nend\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler takes no default")
	err = cameo.Run("t.cameo", "proc :handler\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler declares a field, which only a class body can do")
	err = cameo.Run("t.cameo", "class A\n  def m\n    proc :handler\n  end\nend\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "proc :handler declares a field, which only a class body can do")
}

func TestAProcFieldIsAClosureInTheMessage(t *testing.T) {
	// The class is a message whose field is a cameo.Closure, and an
	// instance reads back from its message with a proc that runs the
	// same code over the values it captured.
	out := runScript(t, `
class Route
  string :path
  proc :handler
end
class App
  list :routes, of: Route
end
puts Route.proto, App.proto
k = 10
r = Route.new(path: "/a", handler: proc { |x| x * k })
back = Route.from_proto(r.to_proto)
p back.path, back.handler.call(4), back.handler.lambda?
p Route.from_proto(Route.new(path: "/n").to_proto)
`)
	assert.Equals(t, out, `message Route {
  optional string path = 36596;
  optional cameo.Closure handler = 55264;
}
message App {
  repeated Route routes = 23606;
}
"/a"
40
false
Route(path: "/n", handler: nil)
`)
}

func TestAProcFieldCrossesTheEnvelopeAndAsync(t *testing.T) {
	// Through a list, a map value, a set and an inherited field; a
	// worker returns an instance with a proc and takes one.
	out := runScript(t, `
class Route
  string :path
  proc :handler
end
class Sub < Route
end
class App
  list :routes, of: Route
  map :named, key: :string, value: Route
  set :pool, of: Route
end
include Cameo
class W
  def make(n) = Route.new(path: "/w", handler: proc { |x| x + n })
  def run(route, x) = route.handler.call(x)
end
r = Route.new(path: "/a", handler: proc { |x| x * 10 })
s = Sub.new(path: "/s", handler: ->(x) { x - 1 })
app = App.new(routes: [r, s], named: {"a" => r}, pool: Set.new([s]))
again = from_value_proto(app.to_value_proto)
p again.routes.map { |x| x.handler.call(5) }, again.named["a"].handler.call(2)
p again.pool.to_a[0].handler.call(1), again.routes[1].handler.lambda?
sub = from_value_proto(s.to_value_proto)
p sub.class, sub.handler.call(3)
p (await async(W.new, :make, 3)).handler.call(4)
p await async(W.new, :run, r, 6)
`)
	assert.Equals(t, out, "[50, 4]\n20\n0\ntrue\nSub\n2\n7\n60\n")
}

func TestAProcFieldThatCannotTravelIsNamed(t *testing.T) {
	// A proc that captured what the envelope cannot carry does not
	// travel: the instance that holds it in a field cannot cross, and
	// the error says which field and why.
	out := runScript(t, `
class Route
  proc :handler
end
include Cameo
class W
  def run(route) = 1
end
each = [1].each
r = Route.new(handler: proc { each.next })
[-> { r.to_proto }, -> { r.to_value_proto }, -> { async(W.new, :run, r) }].each do |f|
  f.call
rescue CameoError, TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, `Route.handler: a Proc that captured each, an Enumerator cannot be serialised
Route.handler: a Proc that captured each, an Enumerator cannot be serialised
a Proc that captured each, an Enumerator cannot be passed to async
`)
}

func TestAnInstanceWithAProcFieldGoesThroughADump(t *testing.T) {
	// A paused script's dump holds the instance, and the resumed run
	// calls its proc.
	const src = "class Route\n  string :path\n  proc :handler\nend\nk = 3\nr = Route.new(path: \"/a\", handler: proc { |x| x * k })\nTest.pause\nputs r.path, r.handler.call(5)\n"
	prog, err := cameo.Load("t.cameo", src, &bytes.Buffer{}, &bytes.Buffer{})
	assert.NoError(t, err)
	prog.Module("Test").Def("pause", func([]any, map[string]any, *cameo.Block) (any, error) {
		prog.Pause()
		return nil, nil
	})
	err = prog.Run()
	assert.True(t, errors.Is(err, cameo.ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, len(dump.Missing), 0)
	var out bytes.Buffer
	cont, err := cameo.Resume(dump.Data, &out, &out)
	assert.NoError(t, err)
	cont.Module("Test").Def("pause", func([]any, map[string]any, *cameo.Block) (any, error) { return nil, nil })
	assert.NoError(t, cont.Run())
	assert.Equals(t, out.String(), "/a\n15\n")
}
