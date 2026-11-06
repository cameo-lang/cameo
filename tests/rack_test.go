package tests

import (
	"bytes"
	"os"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// TestRackMatchesRack runs the scripts in testdata/rack and holds each to
// what Rack 3.2.8 printed under Ruby 3.4.1 for it: Utils, Request,
// Response, URLMap, the middleware, MockRequest and Builder.
func TestRackMatchesRack(t *testing.T) {
	for _, name := range []string{"compat", "builder"} {
		t.Run(name, func(t *testing.T) {
			script := "testdata/rack/" + name + ".cameo"
			expected, err := os.ReadFile("testdata/rack/" + name + ".expected")
			assert.NoError(t, err)
			var stdout, stderr bytes.Buffer
			assert.NoError(t, cameo.RunFile(script, &stdout, &stderr))
			assert.Equals(t, stdout.String(), string(expected))
			assert.Equals(t, stderr.String(), "")
		})
	}
}

// TestRackMiddlewareIsAClassWithAProcField proves a middleware a script
// writes, as Rack's are written in cameo, stacks with use: the app it
// wraps comes as a lambda, which a proc field holds; that the arguments
// use gives reach new; and that what Builder needs is refused plainly.
func TestRackMiddlewareIsAClassWithAProcField(t *testing.T) {
	out := runScript(t, `require "rack"
class Stamp
  proc :app
  string :name

  def initialize(app, name)
    self.app = app
    self.name = name
  end

  def call(env)
    status, headers, body = app.call(env.merge("stamps" => env.fetch("stamps", []) + [name]))
    [status, headers.merge("x-#{name}" => "1"), body]
  end
end
inner = ->(env) { [200, {}, [env["stamps"].join(",")]] }
app = Rack::Builder.new { |b| b.use(Stamp, "outer").use(Stamp, "inner").run(inner) }
res = Rack::MockRequest.new(app).get("/")
p res.body, res.headers
p Rack::Builder.app { |b| b.run { |env| [200, {}, ["block"]] } }.call({})[2]
[-> { Rack::Builder.new { |b| b.use(Stamp, "x") }.to_app }, -> { Rack::Builder.new { |b| 1 } }, -> { Rack::Builder.new.run(42) }, -> { Rack::URLMap.new({"x" => inner}) }, -> { Rack::MockResponse.new(200, {}, "s") }].each do |f|
  f.call
rescue => e
  p [e.class, e.message]
end
`)
	assert.Equals(t, out, `"outer,inner"
{"x-inner" => "1", "x-outer" => "1"}
["block"]
[RuntimeError, "missing run or map statement"]
[TypeError, "Rack::Builder.new's block returns the builder it configured, not Integer"]
[ArgumentError, "a Rack application responds to call; 42 does not"]
[ArgumentError, "paths need to start with /"]
[TypeError, "a Rack body responds to each; String does not, so wrap a String in an Array"]
`)
}
