package tests

import (
	"bytes"
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
	assert.Contains(t, err.Error(), "proc takes a block and nothing else")
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
a Route with a proc field cannot be serialised
a worker returned a Route with a proc field, which cannot be passed back
`)
}

func TestProtoFilesLeaveOutAClassWithAProcField(t *testing.T) {
	prog, err := cameo.Load("t.cameo", "class Route\n  proc :handler\nend\nclass Point\n  int :x\nend\n", &bytes.Buffer{}, &bytes.Buffer{})
	assert.NoError(t, err)
	assert.NoError(t, prog.Run())
	files := prog.ProtoFiles()
	assert.Contains(t, files["cameo.proto"], "message Point")
	assert.NotContains(t, files["cameo.proto"], "message Route")
	_, err = prog.Proto("Route")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Route is no message")
}
