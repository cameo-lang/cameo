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
// expected output beside it. The test allows no I/O, so app.serve says
// so and returns instead of serving.
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
			assert.Equals(t, stderr.String(), "frankie does not serve: this program allows no I/O\n")
		})
	}
}

// TestFrankieServesOverHTTP proves an app's routes answer over the
// network as they do to app.request: the path, the method, the query,
// the headers, the status, a 404 and a 500.
func TestFrankieServesOverHTTP(t *testing.T) {
	port := freePort(t)
	stdout, stderr, stop := serve(t, fmt.Sprintf(`require "frankie"
app = Frankie.app
app = app.get("/hi/:name") { |req| "hi #{req.params["name"]}#{req.params["x"]}" }
app = app.post("/made") { |req| [201, {"X-Agent" => req.headers["User-Agent"]}, req.body] }
app = app.get("/boom") { nil.boom }
app.serve(port: %d, quiet: true)
`, port), port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	res, body := get(t, base+"/hi/sinatra?x=!")
	assert.Equals(t, res.StatusCode, 200)
	assert.Equals(t, body, "hi sinatra!")
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
	assert.Equals(t, body, "serve.cameo:5: undefined method 'boom' for nil (NoMethodError)\n")
	err = stop()
	assert.True(t, err == nil || err == context.Canceled, "the run ended with %v", err)
	assert.Equals(t, stdout.String(), fmt.Sprintf("frankie serves on http://127.0.0.1:%d\n", port))
	assert.Equals(t, stderr.String(), "serve.cameo:5: undefined method 'boom' for nil (NoMethodError)\n")
}

// TestFrankieResponseFormsAndErrors proves app.request reads every form a
// route may return, and raises the route's exception to the caller.
func TestFrankieResponseFormsAndErrors(t *testing.T) {
	out := runScript(t, `require "frankie"
app = Frankie.app
app = app.get("/s") { "body" }
app = app.get("/i") { 204 }
app = app.get("/n") { nil }
app = app.get("/two") { [202, "two"] }
app = app.get("/bad") { [1, 2, 3, 4] }
app = app.get("/hash") { {} }
app = app.get("/raise") { raise ArgumentError, "inside" }
%w[/s /i /n /two].each { |path| r = app.request("GET", path); p [r.status, r.headers, r.body] }
%w[/bad /hash /raise].each do |path|
  app.request("GET", path)
rescue TypeError, ArgumentError => e
  puts e.message
end
p app.routes.map(&:path), app.port, app.set(port: 1, quiet: true).quiet
`)
	assert.Equals(t, out, `[200, {"Content-Type" => "text/html;charset=utf-8"}, "body"]
[204, {"Content-Type" => "text/html;charset=utf-8"}, ""]
[200, {"Content-Type" => "text/html;charset=utf-8"}, ""]
[202, {"Content-Type" => "text/html;charset=utf-8"}, "two"]
a route returned an Array of 4, not [status, body] or [status, headers, body]
a route returned {}, not a String, an Integer or an Array
inside
["/s", "/i", "/n", "/two", "/bad", "/hash", "/raise"]
4567
true
`)
}
