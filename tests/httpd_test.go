package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// freePort returns a port nothing listens on right now.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// serve runs src, which calls HTTPD.serve on port, in a program that
// allows io/httpd, until stop is called; it returns once the server
// answers, with what the script wrote and a stop that returns the run's
// error.
func serve(t *testing.T, src string, port int) (stdout, stderr *bytes.Buffer, stop func() error) {
	t.Helper()
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	prog, err := cameo.Load("serve.cameo", src, stdout, stderr)
	assert.NoError(t, err)
	prog.AllowIO("io/httpd")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- prog.RunContext(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the script ended before serving: %v\nstderr: %s", err, stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("nothing listens on %d after 5s\nstderr: %s", port, stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	return stdout, stderr, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("the script did not end after the run was stopped")
			return nil
		}
	}
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	res, err := http.Get(url)
	assert.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	return res, string(body)
}

// TestHTTPDServesARackApp proves the server gives an app a Rack env, the
// method, the path as it came, the query string, the headers as HTTP_*,
// the body as rack.input, and writes the [status, headers, body] it
// returns: an Array body or one whose each yields, a header's lines or
// Array its values each, a StandardError a 500.
func TestHTTPDServesARackApp(t *testing.T) {
	port := freePort(t)
	_, stderr, stop := serve(t, fmt.Sprintf(`require "io/httpd"
class Chunks
  def each
    yield "a"
    yield "b"
  end
end
class App
  def call(env)
    case env["PATH_INFO"]
    when "/" then [200, {"content-type" => "text/plain"}, ["hello"]]
    when "/full" then [202, {"x-q" => env["QUERY_STRING"], "x-two" => "1\n2", "x-list" => ["3", "4"]}, ["#{env["REQUEST_METHOD"]} #{env["HTTP_USER_AGENT"]} #{env["CONTENT_TYPE"]} #{env["rack.input"].read} #{env["SERVER_NAME"]} #{env["SCRIPT_NAME"].inspect}"]]
    when "/each" then [200, {}, Chunks.new]
    when "/a%%20b" then [200, {}, [env["PATH_INFO"]]]
    when "/boom" then nil.boom
    else [404, {}, ["no #{env["PATH_INFO"]}"]]
    end
  end
end
HTTPD.serve(App.new, port: %d, quiet: true)
puts "served"
`, port), port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	res, body := get(t, base+"/")
	assert.Equals(t, res.StatusCode, 200)
	assert.Equals(t, body, "hello")
	assert.Equals(t, res.Header.Get("Content-Type"), "text/plain")
	req, err := http.NewRequest("POST", base+"/full?q=1", strings.NewReader("ring"))
	assert.NoError(t, err)
	req.Header.Set("User-Agent", "t")
	req.Header.Set("Content-Type", "text/x")
	res, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	full, _ := io.ReadAll(res.Body)
	res.Body.Close()
	assert.Equals(t, res.StatusCode, 202)
	assert.Equals(t, res.Header.Get("X-Q"), "q=1")
	assert.Equals(t, res.Header.Values("X-Two"), []string{"1", "2"})
	assert.Equals(t, res.Header.Values("X-List"), []string{"3", "4"})
	assert.Equals(t, string(full), `POST t text/x ring 127.0.0.1 ""`)
	_, body = get(t, base+"/each")
	assert.Equals(t, body, "ab")
	_, body = get(t, base+"/a%20b")
	assert.Equals(t, body, "/a%20b")
	res, body = get(t, base+"/nope")
	assert.Equals(t, res.StatusCode, 404)
	assert.Equals(t, body, "no /nope")
	res, body = get(t, base+"/boom")
	assert.Equals(t, res.StatusCode, 500)
	assert.Equals(t, body, "serve.cameo:15: undefined method 'boom' for nil (NoMethodError)\n")
	assert.Equals(t, stderr.String(), "serve.cameo:15: undefined method 'boom' for nil (NoMethodError)\n")
	err = stop()
	assert.True(t, err == nil || err == context.Canceled, "the run ended with %v", err)
}

// TestHTTPDServesABlockAndARackBuilder proves a block is an app, given the
// env, and that a Rack::Builder with middleware serves as Rack::MockRequest
// sees it.
func TestHTTPDServesABlockAndARackBuilder(t *testing.T) {
	port := freePort(t)
	_, _, stop := serve(t, fmt.Sprintf(`require "io/httpd"
require "rack"
inner = ->(env) { [200, {"content-type" => "text/plain"}, ["#{env["SCRIPT_NAME"]}|#{env["PATH_INFO"]}|#{Rack::Request.new(env).params["x"]}"]] }
app = Rack::Builder.new { |b| b.use(Rack::ContentLength).map("/admin") { |a| a.run(inner) }.run(inner) }
HTTPD.serve(port: %d, quiet: true) { |env| app.call(env) }
`, port), port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	res, body := get(t, base+"/admin/users?x=1")
	assert.Equals(t, body, "/admin|/users|1")
	assert.Equals(t, res.Header.Get("Content-Length"), "15")
	_, body = get(t, base+"/other")
	assert.Equals(t, body, "|/other|")
	err := stop()
	assert.True(t, err == nil || err == context.Canceled, "the run ended with %v", err)
}

func TestHTTPDLogsEachRequestUnlessQuiet(t *testing.T) {
	port := freePort(t)
	_, stderr, stop := serve(t, fmt.Sprintf(`require "io/httpd"
HTTPD.serve(port: %d) { |env| [200, {}, ["hi"]] }
`, port), port)
	get(t, fmt.Sprintf("http://127.0.0.1:%d/hi/you?x=1", port))
	stop()
	assert.Matches(t, stderr.String(), `^127\.0\.0\.1 - - \[\d\d/[A-Z][a-z]{2}/\d{4}:\d\d:\d\d:\d\d [-+]\d{4}\] "GET /hi/you\?x=1 HTTP/1\.1" 200 2 0\.\d{4}\n$`)
}

func TestHTTPDRefusesToServeWhereTheHostAllowsNoIO(t *testing.T) {
	out := runScript(t, `require "io/httpd"
begin
  HTTPD.serve(port: 1) { |env| [200, {}, ["hi"]] }
rescue HTTPD::Error => e
  puts e.message
end
[-> { HTTPD.serve(port: "x") { |env| [200, {}, []] } }, -> { HTTPD.serve(42) }, -> { HTTPD.serve }, -> { HTTPD.serve(->(env) {}) { |env| } }].each do |f|
  f.call
rescue TypeError, ArgumentError => e
  puts e.message
end
`)
	assert.Equals(t, out, "the host allows no network I/O: run the script with cameo --io=network, or let the program say AllowIO(\"network\") or AllowIO(\"io/httpd\")\nport must be an Integer, not String\na Rack app responds to call; 42 does not\nHTTPD.serve takes a Rack app, or a block that answers each request's env\nHTTPD.serve takes an app or a block, not both\n")
}

func TestHTTPDReportsASocketItCannotOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("busy.cameo", fmt.Sprintf(`require "io/httpd"
begin
  HTTPD.serve(port: %d) { |env| [200, {}, ["hi"]] }
rescue HTTPD::Error => e
  puts e.message.include?("address already in use")
end
`, port), &stdout, &stderr)
	assert.NoError(t, err)
	prog.AllowIO("io/httpd")
	assert.NoError(t, prog.Run())
	assert.Equals(t, stdout.String(), "true\n")
}

func TestHTTPDAllowedFollowsTheHost(t *testing.T) {
	assert.Equals(t, runScript(t, `require "io/httpd"; p HTTPD.allowed?`), "false\n")
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", `require "io/httpd"; p HTTPD.allowed?`, &stdout, &stderr)
	assert.NoError(t, err)
	prog.AllowIO("io/httpd")
	assert.NoError(t, prog.Run())
	assert.Equals(t, stdout.String(), "true\n")
}

// A kind of I/O allowed, network, allows the library; another kind, or
// nothing, does not; all does.
func TestHTTPDIsAllowedByItsKindOfIO(t *testing.T) {
	for _, c := range []struct {
		what string
		want string
	}{{"network", "true\n"}, {"file", "false\n"}, {"all", "true\n"}, {"", "false\n"}} {
		var stdout, stderr bytes.Buffer
		prog, err := cameo.Load("t.cameo", `require "io/httpd"; p HTTPD.allowed?`, &stdout, &stderr)
		assert.NoError(t, err)
		if c.what != "" {
			prog.AllowIO(c.what)
		}
		assert.NoError(t, prog.Run())
		assert.Equals(t, stdout.String(), c.want, "AllowIO(%q)", c.what)
	}
}

// A break in the block ends serve and the script goes on; a repeated
// request header is joined, cookies with semicolons, and Host is
// HTTP_HOST; a status below 200, a String body and a reply that is no
// triple are refused.
func TestHTTPDEndsOnABreakAndReadsHeadersAsRackDoes(t *testing.T) {
	port := freePort(t)
	stdout, stderr, stop := serve(t, fmt.Sprintf(`require "io/httpd"
HTTPD.serve(port: %d, quiet: true) do |env|
  case env["PATH_INFO"]
  when "/stop" then break
  when "/early" then [101, {}, []]
  when "/string" then [200, {}, "x"]
  when "/pair" then [200, "x"]
  else [200, {}, ["#{env["HTTP_HOST"]} #{env["HTTP_X_MANY"]} #{env["HTTP_COOKIE"]}"]]
  end
end
puts "after"
`, port), port)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/h", port), nil)
	assert.NoError(t, err)
	req.Header.Add("X-Many", "a")
	req.Header.Add("X-Many", "b")
	req.Header.Add("Cookie", "c=1")
	req.Header.Add("Cookie", "d=2")
	res, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	joined, _ := io.ReadAll(res.Body)
	res.Body.Close()
	assert.Equals(t, string(joined), fmt.Sprintf("127.0.0.1:%d a, b c=1; d=2", port))
	for path, want := range map[string]string{
		"/early":  "not an Integer from 200 to 999",
		"/string": "a Rack body responds to each; String does not, so wrap a String in an Array",
		"/pair":   "a Rack app returns [status, headers, body], not [200, \"x\"]",
	} {
		res, body := get(t, fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
		assert.Equals(t, res.StatusCode, 500)
		assert.True(t, strings.Contains(body, want), body)
	}
	res, _ = get(t, fmt.Sprintf("http://127.0.0.1:%d/stop", port))
	assert.Equals(t, res.StatusCode, 503)
	assert.NoError(t, stop())
	assert.Equals(t, stdout.String(), "after\n")
	assert.True(t, strings.Contains(stderr.String(), "a Rack app returned the status 101, not an Integer from 200 to 999 (TypeError)"), stderr.String())
}
