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

// TestRackRequestHoldsItsEnvInGo proves the part of Rack::Request Go
// defines, new(env), env, get_header and has_header?, works with what
// rack.cameo adds to the class: a subclass's new makes the subclass, a
// request's env is the Hash it was given, and an env that is no Hash is
// refused.
func TestRackRequestHoldsItsEnvInGo(t *testing.T) {
	out := runScript(t, `require "rack"
class Mine < Rack::Request
  def shout = path.upcase
end
env = {"REQUEST_METHOD" => "GET", "PATH_INFO" => "/a", "QUERY_STRING" => "x=1"}
req = Mine.new(env)
p req.class, req.is_a?(Rack::Request), req.shout, req.GET
p req.env == env, req.get_header("PATH_INFO"), req.get_header("NONE"), req.has_header?("PATH_INFO"), req.has_header?("NONE")
p req.fetch_header("NONE") { |k| k.downcase }
begin
  Rack::Request.new("GET /")
rescue TypeError => e
  p e.message
end
`)
	assert.Equals(t, out, `Mine
true
"/A"
{"x" => "1"}
true
"/a"
nil
true
false
"none"
"a Rack env is a Hash, not String"
`)
}

// TestRackHandlerRegistersTheServersOfTheProgram proves Rack::Handler keeps
// the servers registered, by name and in order: get finds one, default
// returns the first that can serve, passing over one whose allowed? is
// false, a server registered again keeps its place, what does not run an
// app is refused, and another program starts with none.
func TestRackHandlerRegistersTheServersOfTheProgram(t *testing.T) {
	out := runScript(t, `require "rack"
module Closed
  def self.allowed? = false
  def self.run(app, **options) = "closed"
end
module Open
  def self.run(app, **options) = "#{options[:Host]}:#{options[:Port]} #{app.call({})[2].first}"
end
p Rack::Handler.default, Rack::Handler.names
Rack::Handler.register("closed", Closed)
p Rack::Handler.default
Rack::Handler.register(:open, Open)
Rack::Handler.register("closed", Closed)
p Rack::Handler.names, Rack::Handler.get("open"), Rack::Handler.get(:none)
p Rack::Handler.default.run(->(env) { [200, {}, ["hi"]] }, Host: "localhost", Port: 9292)
begin
  Rack::Handler.register("x", 42)
rescue ArgumentError => e
  p e.message
end
`)
	assert.Equals(t, out, `nil
[]
nil
["closed", "open"]
Open
nil
"localhost:9292 hi"
"a Rack handler responds to run; 42 does not"
`)
	assert.Equals(t, runScript(t, "require \"rack\"\np Rack::Handler.names\n"), "[]\n")
}
