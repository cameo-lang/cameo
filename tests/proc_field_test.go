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

func TestAClassWithAProcFieldIsNoMessage(t *testing.T) {
	out := runScript(t, `
class Route
  proc :handler
end
class App
  list :routes, of: Route
end
include Cameo
class W
  def make = Route.new(handler: proc { 1 })
end
[-> { Route.new.to_proto }, -> { App.proto }, -> { Route.new.to_value_proto }, -> { await async(W.new, :make) }].each do |f|
  f.call
rescue CameoError, TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, `Route is no message: its field handler is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
App is no message: its field handler through Route is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
Route is no message: its field handler is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
a worker returned a Route with a proc field, which cannot be passed back
`)
}

func TestAProcFieldIsFoundThroughMapsSetsAndInheritance(t *testing.T) {
	// The article follows the class name, and the field is found through
	// a map value, a set element and an inherited field.
	out := runScript(t, `
class App
  proc :handler
end
class ByName
  map :apps, key: :string, value: App
end
class Pool
  set :apps, of: App
end
class Sub < App
end
include Cameo
class W
  def make = App.new
end
[-> { ByName.new.to_proto }, -> { Pool.new.to_proto }, -> { Sub.new.to_proto }, -> { async(App.new, :handler) }, -> { await async(W.new, :make) }].each do |f|
  f.call
rescue CameoError, TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, `ByName is no message: its field handler through App is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
Pool is no message: its field handler through App is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
Sub is no message: its field handler is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump
an App with a proc field cannot be passed to async
a worker returned an App with a proc field, which cannot be passed back
`)
}

func TestADumpLeavesOutAnInstanceWithAProcField(t *testing.T) {
	// A paused script's dump reports the instance missing, with the
	// explanation, as it reports a Proc.
	prog, err := cameo.Load("t.cameo", "class Route\n  proc :handler\nend\nr = Route.new(handler: proc { 1 })\nn = 1\nTest.pause\nn = 2\n", &bytes.Buffer{}, &bytes.Buffer{})
	assert.NoError(t, err)
	prog.Module("Test").Def("pause", func([]any, map[string]any) (any, error) {
		prog.Pause()
		return nil, nil
	})
	err = prog.Run()
	assert.True(t, errors.Is(err, cameo.ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, dump.Missing, []string{"r (Route is no message: its field handler is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump)"})
}
