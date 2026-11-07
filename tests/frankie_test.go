package tests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// TestFrankieExamplesPassTheirSpecs runs each example under
// examples/frankie, whose specs run as part of it, and holds it to the
// expected output beside it. The test allows no I/O, so io/httpd, which
// each requires, may not serve, and app.serve says so and returns.
func TestFrankieExamplesPassTheirSpecs(t *testing.T) {
	scripts, err := filepath.Glob("../examples/frankie/*.cameo")
	assert.NoError(t, err)
	assert.True(t, len(scripts) > 0, "no scripts in examples/frankie")
	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			expected, err := os.ReadFile(strings.TrimSuffix(script, ".cameo") + ".expected")
			assert.NoError(t, err, "missing expected output")
			var stdout, stderr bytes.Buffer
			assert.NoError(t, cameo.RunFile(script, &stdout, &stderr))
			assert.Equals(t, stdout.String(), string(expected))
			assert.Equals(t, stderr.String(), "frankie does not serve: this program allows none of its Rack servers to serve (httpd)\n")
		})
	}
}

// TestFrankieServesOverHTTP proves an app's routes answer over the
// network as they do to Rack::MockRequest: the path, the method, the
// query, the headers, the body, the status, a 404 and a 500.
func TestFrankieServesOverHTTP(t *testing.T) {
	port := freePort(t)
	stdout, stderr, stop := serve(t, fmt.Sprintf(`require "frankie"
require "io/httpd"
app = Frankie.app
app = app.get("/hi/:name") { |req| "hi #{req.params["name"]}#{req.params["x"]}" }
app = app.post("/made") { |req| [201, {"X-Agent" => req.user_agent}, req.body.read] }
app = app.get("/boom") { nil.boom }
app.serve(port: %d, quiet: true)
`, port), port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	res, body := get(t, base+"/hi/sina%20tra?x=!")
	assert.Equals(t, res.StatusCode, 200)
	assert.Equals(t, body, "hi sina tra!")
	assert.Equals(t, res.Header.Get("Content-Type"), "text/html;charset=utf-8")
	req, err := http.NewRequest("POST", base+"/made", strings.NewReader("ring"))
	assert.NoError(t, err)
	req.Header.Set("User-Agent", "t")
	post, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	made := new(bytes.Buffer)
	made.ReadFrom(post.Body)
	post.Body.Close()
	assert.Equals(t, post.StatusCode, 201)
	assert.Equals(t, post.Header.Get("X-Agent"), "t")
	assert.Equals(t, made.String(), "ring")
	res, body = get(t, base+"/missing")
	assert.Equals(t, res.StatusCode, 404)
	assert.Equals(t, body, "Not Found")
	res, body = get(t, base+"/boom")
	assert.Equals(t, res.StatusCode, 500)
	assert.Equals(t, body, "serve.cameo:6: undefined method 'boom' for nil (NoMethodError)\n")
	err = stop()
	assert.True(t, err == nil || err == context.Canceled, "the run ended with %v", err)
	assert.Equals(t, stdout.String(), fmt.Sprintf("frankie serves on http://127.0.0.1:%d\n", port))
	assert.Equals(t, stderr.String(), "serve.cameo:6: undefined method 'boom' for nil (NoMethodError)\n")
}

// TestFrankieResponseFormsAndErrors proves an app reads every form a route
// may return, as Rack's triple with lowercase header names, and raises the
// route's exception to the caller.
func TestFrankieResponseFormsAndErrors(t *testing.T) {
	out := runScript(t, `require "frankie"
app = Frankie.app
app = app.get("/s") { "body" }
app = app.get("/i") { 204 }
app = app.get("/n") { nil }
app = app.get("/two") { [202, "two"] }
app = app.get("/three") { [200, {"X-A" => "b", "Content-Type" => "text/plain"}, ["x", "y"]] }
app = app.get("/bad") { [1, 2, 3, 4] }
app = app.get("/hash") { {} }
app = app.get("/raise") { raise ArgumentError, "inside" }
browser = Rack::MockRequest.new(app)
%w[/s /i /n /two /three].each { |path| r = browser.get(path); p [r.status, r.headers, r.body] }
p app.call(Rack::MockRequest.env_for("/s"))
%w[/bad /hash /raise].each do |path|
  browser.get(path)
rescue TypeError, ArgumentError => e
  puts e.message
end
p app.routes.map(&:path), app.port, app.set(port: 1, quiet: true).quiet
`)
	assert.Equals(t, out, `[200, {"content-type" => "text/html;charset=utf-8"}, "body"]
[204, {"content-type" => "text/html;charset=utf-8"}, ""]
[200, {"content-type" => "text/html;charset=utf-8"}, ""]
[202, {"content-type" => "text/html;charset=utf-8"}, "two"]
[200, {"x-a" => "b", "content-type" => "text/plain"}, "xy"]
[200, {"content-type" => "text/html;charset=utf-8"}, ["body"]]
a route returned an Array of 4, not [status, body] or [status, headers, body]
a route returned Hash, not a String, an Integer or an Array
inside
["/s", "/i", "/n", "/two", "/three", "/bad", "/hash", "/raise"]
4567
true
`)
}

// A spec sees what the wire sees: a percent-decoded capture, a status
// outside 200..999 refused, a route without a block refused at the
// definition, set taking its three settings alone, a trailing or doubled
// slash matching no route, HEAD as GET with an empty body, a query string
// that is not one as a 400; and the request is a Rack::Request, whose
// params, form and cookies a route reads as Sinatra's does.
func TestFrankieSpecsSeeWhatTheWireSees(t *testing.T) {
	out := runScript(t, `require "frankie"
app = Frankie.app
app = app.get("/hi/:name") { |r| r.params["name"] }
app = app.get("/x") { 150 }
app = app.get("/h") { "body" }
app = app.post("/form/:id") { |r| [r.class, r.is_a?(Rack::Request), r.params, r.POST, r.cookies, r.path_info, r.url].inspect }
browser = Rack::MockRequest.new(app)
p browser.get("/hi/a%20b").body
begin; browser.get("/x"); rescue TypeError => e; puts e.message; end
begin; app.get("/y"); rescue ArgumentError => e; puts e.message; end
begin; app.set(bogus: 1); rescue ArgumentError => e; puts e.message; end
p app.set(port: 1).routes.size, browser.get("/hi/bob/").status, browser.get("//hi//bob").status, browser.head("/h").body, browser.head("/h").status
p browser.get("/h", "QUERY_STRING" => "a=%zz").status, browser.get("/h", "QUERY_STRING" => "a=%zz").body
puts browser.post("/form/7?q=1", params: {"f" => "2"}, "HTTP_COOKIE" => "s=x").body
wrapped = Rack::Builder.new { |b| b.use(Rack::ContentLength).run(app) }
p Rack::MockRequest.new(wrapped).get("/h").headers
`)
	assert.Equals(t, out, `"a b"
a route returned the status 150, not an Integer from 200 to 999
get("/y") takes a block that returns the response
unknown keyword: :bogus
4
404
404
""
200
400
"Bad Request"
[Frankie::Request, true, {"q" => "1", "f" => "2", "id" => "7"}, {"f" => "2"}, {"s" => "x"}, "/form/7", "http://example.org/form/7?q=1"]
{"content-type" => "text/html;charset=utf-8", "content-length" => "4"}
`)
}

// TestFrankieServesOnTheDefaultRackHandler proves serve runs the app on
// Rack::Handler.default with rackup's options from the app's settings,
// whatever server registered, and returns false, saying why, where no
// server is loaded.
func TestFrankieServesOnTheDefaultRackHandler(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `require "frankie"
module Fake
  def self.run(app, **options)
    yield "fake://#{options[:Host]}:#{options[:Port]}"
    puts app.call({"REQUEST_METHOD" => "GET", "PATH_INFO" => "/"})[2].first, options[:Quiet]
  end
end
app = Frankie.app.get("/") { "served" }
p app.serve
Rack::Handler.register("fake", Fake)
p app.serve(port: 8080, quiet: true)
`, &stdout, &stderr)
	assert.NoError(t, err)
	assert.Equals(t, stdout.String(), "false\nfrankie serves on fake://127.0.0.1:8080\nserved\ntrue\ntrue\n")
	assert.Equals(t, stderr.String(), "frankie does not serve: no Rack server is loaded; require one\n")
}
