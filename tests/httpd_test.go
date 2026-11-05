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

func TestHTTPDServesTheBlocksAnswersInSinatrasForms(t *testing.T) {
	port := freePort(t)
	_, stderr, stop := serve(t, fmt.Sprintf(`require "io/httpd"
HTTPD.serve(port: %d, quiet: true) do |req|
  case req.path
  when "/" then "hello"
  when "/pair" then [201, "made"]
  when "/full" then [202, {"Content-Type" => "text/plain", "X-Q" => req.query}, "#{req.method} #{req.headers["User-Agent"]} #{req.body}"]
  when "/status" then 204
  when "/boom" then nil.boom
  else [404, "no #{req.path}"]
  end
end
puts "served"
`, port), port)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	res, body := get(t, base+"/")
	assert.Equals(t, res.StatusCode, 200)
	assert.Equals(t, body, "hello")
	assert.Equals(t, res.Header.Get("Content-Type"), "text/html;charset=utf-8")
	res, body = get(t, base+"/pair")
	assert.Equals(t, res.StatusCode, 201)
	assert.Equals(t, body, "made")
	req, err := http.NewRequest("POST", base+"/full?q=1", strings.NewReader("ring"))
	assert.NoError(t, err)
	req.Header.Set("User-Agent", "t")
	res, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	full, _ := io.ReadAll(res.Body)
	res.Body.Close()
	assert.Equals(t, res.StatusCode, 202)
	assert.Equals(t, res.Header.Get("Content-Type"), "text/plain")
	assert.Equals(t, res.Header.Get("X-Q"), "q=1")
	assert.Equals(t, string(full), "POST t ring")
	res, body = get(t, base+"/status")
	assert.Equals(t, res.StatusCode, 204)
	assert.Equals(t, body, "")
	res, body = get(t, base+"/nope")
	assert.Equals(t, res.StatusCode, 404)
	assert.Equals(t, body, "no /nope")
	res, body = get(t, base+"/boom")
	assert.Equals(t, res.StatusCode, 500)
	assert.Equals(t, body, "serve.cameo:8: undefined method 'boom' for nil (NoMethodError)\n")
	assert.Equals(t, stderr.String(), "serve.cameo:8: undefined method 'boom' for nil (NoMethodError)\n")
	err = stop()
	assert.True(t, err == nil || err == context.Canceled, "the run ended with %v", err)
}

func TestHTTPDLogsEachRequestUnlessQuiet(t *testing.T) {
	port := freePort(t)
	_, stderr, stop := serve(t, fmt.Sprintf(`require "io/httpd"
HTTPD.serve(port: %d) { |req| "hi" }
`, port), port)
	get(t, fmt.Sprintf("http://127.0.0.1:%d/hi/you?x=1", port))
	stop()
	assert.Matches(t, stderr.String(), `^127\.0\.0\.1 - - \[\d\d/[A-Z][a-z]{2}/\d{4}:\d\d:\d\d:\d\d [-+]\d{4}\] "GET /hi/you\?x=1 HTTP/1\.1" 200 2 0\.\d{4}\n$`)
}

func TestHTTPDRefusesToServeWhereTheHostAllowsNoIO(t *testing.T) {
	out := runScript(t, `require "io/httpd"
begin
  HTTPD.serve(port: 1) { |req| "hi" }
rescue HTTPD::Error => e
  puts e.message
end
begin
  HTTPD.serve(port: "x") { |req| "hi" }
rescue TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, "the host allows no I/O through io/httpd: HTTPD.serve needs a program that allows it\nport must be an Integer, not String\n")
}

func TestHTTPDReportsASocketItCannotOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("busy.cameo", fmt.Sprintf(`require "io/httpd"
begin
  HTTPD.serve(port: %d) { |req| "hi" }
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
