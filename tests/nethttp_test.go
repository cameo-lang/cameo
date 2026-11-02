package tests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// testServer serves what the net/http tests fetch: a greeting with
// headers, JSON, an echo of the request, and the status codes a script
// handles.
func testServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Test", "one")
		w.Header().Add("X-Multi", "a")
		w.Header().Add("X-Multi", "b")
		fmt.Fprint(w, "hello, world")
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var keys []string
		for k := range r.Header {
			if k != "Accept-Encoding" && k != "User-Agent" && k != "Accept" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		fmt.Fprintf(w, "%s %s host=%s", r.Method, r.URL.RequestURI(), r.Host)
		for _, k := range keys {
			fmt.Fprintf(w, " %s: %s", k, strings.Join(r.Header[k], ", "))
		}
		fmt.Fprintf(w, " body=%q", body)
	})
	mux.HandleFunc("/notfound", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", 404) })
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/hello", 302) })
	mux.HandleFunc("/nocontent", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		fmt.Fprint(w, "late")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// runNetScript runs src with the server's address as BASE and network
// access through client, and returns what it printed.
func runNetScript(t *testing.T, client *http.Client, base, src string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", "require \"net/http\"\nBASE = \""+base+"\"\n"+src, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	prog.HTTPClient(client)
	if err := prog.Run(); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String()
}

func TestNetHTTPGetsAndReadsTheResponse(t *testing.T) {
	srv := testServer(t)
	out := runNetScript(t, srv.Client(), srv.URL, `
u = URI("#{BASE}/hello")
p Net::HTTP.get(u), Net::HTTP.get(u).encoding, Net::HTTP.get(u.host, "/hello", u.port)
res = Net::HTTP.get_response(u)
p [res.class, res.code, res.message, res.body, res["content-type"], res["X-Test"], res["x-multi"], res.get_fields("x-multi"), res.content_type, res.key?("x-test"), res["nope"]]
p [res.is_a?(Net::HTTPSuccess), res.code_type, res.value, res.http_version, res.uri.path, res.content_length, res.main_type, res.sub_type, res.type_params, res.charset]
p res, res.to_hash["x-multi"], res.each_header.to_a.sort.first(2), res.each_capitalized.to_a.sort.first(1)
p Net::HTTP.get_response(u) { |r| r.code }
`)
	assert.Equals(t, out, `"hello, world"
#<Encoding:UTF-8>
"hello, world"
[Net::HTTPOK, "200", "OK", "hello, world", "text/plain; charset=utf-8", "one", "a, b", ["a", "b"], "text/plain", true, nil]
[true, Net::HTTPOK, nil, "1.1", "/hello", 12, "text", "plain", {"charset" => "utf-8"}, "utf-8"]
#<Net::HTTPOK 200 OK readbody=true>
["a", "b"]
[["content-length", "12"], ["content-type", "text/plain; charset=utf-8"]]
[["Content-Length", "12"]]
#<Net::HTTPOK 200 OK readbody=true>
`)
}

func TestNetHTTPResponseClassesFollowTheStatus(t *testing.T) {
	srv := testServer(t)
	out := runNetScript(t, srv.Client(), srv.URL, `
res = Net::HTTP.get_response(URI("#{BASE}/notfound"))
p [res.class, res.code, res.body, res.is_a?(Net::HTTPClientError), res.is_a?(Net::HTTPSuccess)]
begin; res.value; rescue Net::HTTPClientException => e; p [e.class, e.message, e.response.class]; end
begin; Net::HTTP.get_response(URI("#{BASE}/fail")).value; rescue => e; p [e.class, e.message]; end
res = Net::HTTP.get_response(URI("#{BASE}/redirect"))
p [res.class, res.code, res["location"]]
begin; res.value; rescue => e; p [e.class, e.message]; end
res = Net::HTTP.get_response(URI("#{BASE}/nocontent"))
p [res.class, res.code, res.body]
case Net::HTTP.get_response(URI("#{BASE}/hello"))
when Net::HTTPSuccess then puts "success"
end
p [Net::HTTPResponse::CODE_TO_OBJ["404"], Net::HTTPNotFound.ancestors.take(4), Net::HTTPClientException.ancestors.take(4), Net::HTTPServerException, Net::ReadTimeout.ancestors.take(3)]
`)
	assert.Equals(t, out, `[Net::HTTPNotFound, "404", "nope\n", true, false]
[Net::HTTPClientException, "404 \"Not Found\"", Net::HTTPNotFound]
[Net::HTTPFatalError, "500 \"Internal Server Error\""]
[Net::HTTPFound, "302", "/hello"]
[Net::HTTPRetriableError, "302 \"Found\""]
[Net::HTTPNoContent, "204", nil]
success
[Net::HTTPNotFound, [Net::HTTPNotFound, Net::HTTPClientError, Net::HTTPResponse, Net::HTTPHeader], [Net::HTTPClientException, Net::HTTPExceptions, Net::ProtoServerError, Net::ProtocolError], Net::HTTPClientException, [Net::ReadTimeout, Timeout::Error, RuntimeError]]
`)
}

// Where Ruby changes a request in place, cameo's set_form_data and
// basic_auth return the changed copy, which the script binds; req["X"] = v
// and req.body = v rebind the variable as every assignment does.
func TestNetHTTPSendsRequestsWithHeadersAndBodies(t *testing.T) {
	srv := testServer(t)
	out := runNetScript(t, srv.Client(), srv.URL, `
u = URI("#{BASE}/echo")
puts Net::HTTP.post(u, '{"a":1}', "Content-Type" => "application/json").body
puts Net::HTTP.post_form(u, "q" => "a b", "n" => "1").body
Net::HTTP.start(u.host, u.port) do |http|
  p [http.class, http.address, http.port, http.use_ssl?, http.started?, http.open_timeout, http.read_timeout]
  puts http.get("/echo", "X-Custom" => "yes").body
  p http.head("/hello").body, http.head("/hello").class
  puts http.post("/echo", "a=1&b=2").body, http.put("/echo", "data", "Content-Type" => "text/plain").body
  puts http.delete("/echo").body, http.patch("/echo", "p").body
  req = Net::HTTP::Get.new("/echo")
  p [req.class, req.method, req.path, req.request_body_permitted?, req.response_body_permitted?, req["accept"], req["user-agent"], req.key?("host")]
  req = Net::HTTP::Post.new(u, "X-A" => "1")
  req["X-B"] = "2"
  req.body = "payload"
  p [req.path, req.uri.class, req["host"], req["x-a"], req.to_hash["x-b"], req.body]
  puts http.request(req).body
  req = Net::HTTP::Post.new("/echo")
  req = req.set_form_data("x" => "1", "y" => "two words")
  p [req.body, req.content_type]
  puts http.request(req).body
  req = Net::HTTP::Get.new("/echo")
  req = req.basic_auth("user", "pass")
  p req["authorization"]
  puts http.request(req).body, http.request_get("/hello").body, http.send_request("GET", "/hello").body
end
`)
	host := strings.TrimPrefix(srv.URL, "http://")
	port := host[strings.LastIndex(host, ":")+1:]
	assert.Equals(t, out, strings.NewReplacer("HOST", host, "PORT", port).Replace(`POST /echo host=HOST Content-Length: 7 Content-Type: application/json body="{\"a\":1}"
POST /echo host=HOST Content-Length: 9 Content-Type: application/x-www-form-urlencoded body="q=a+b&n=1"
[Net::HTTP, "127.0.0.1", PORT, false, true, 60, 60]
GET /echo host=HOST X-Custom: yes body=""
nil
Net::HTTPOK
POST /echo host=HOST Content-Length: 7 Content-Type: application/x-www-form-urlencoded body="a=1&b=2"
PUT /echo host=HOST Content-Length: 4 Content-Type: text/plain body="data"
DELETE /echo host=HOST Depth: Infinity body=""
PATCH /echo host=HOST Content-Length: 1 Content-Type: application/x-www-form-urlencoded body="p"
[Net::HTTP::Get, "GET", "/echo", false, true, "*/*", "Ruby", false]
["/echo", URI::HTTP, "HOST", "1", ["2"], "payload"]
POST /echo host=HOST Content-Length: 7 Content-Type: application/x-www-form-urlencoded X-A: 1 X-B: 2 body="payload"
["x=1&y=two+words", "application/x-www-form-urlencoded"]
POST /echo host=HOST Content-Length: 15 Content-Type: application/x-www-form-urlencoded body="x=1&y=two+words"
"Basic dXNlcjpwYXNz"
GET /echo host=HOST Authorization: Basic dXNlcjpwYXNz body=""
hello, world
hello, world
`))
}

func TestNetHTTPRaisesRubysErrors(t *testing.T) {
	srv := testServer(t)
	out := runNetScript(t, srv.Client(), srv.URL, `
u = URI(BASE)
http = Net::HTTP.new(u.host, u.port)
p [http.started?, http.inspect, http.start.started?]
http.read_timeout = 0.2
begin; http.get("/slow"); rescue Net::ReadTimeout => e; p [e.class, e.message]; end
begin; Net::HTTP.get(URI("http://127.0.0.1:1/x")); rescue SystemCallError => e; p [e.class, e.message]; end
begin; Net::HTTP.get(URI("http://no-such-host.invalid/x")); rescue SocketError => e; p [e.class, e.message]; end
begin; Net::HTTP::Get.new(""); rescue ArgumentError => e; p [e.class, e.message]; end
p [Net::HTTP.new("x.com").port, Net::HTTP.new("x.com", 443).use_ssl?, Net::HTTP.new("x.com", 8080).port]
h = Net::HTTP.new("x.com")
h.use_ssl = true
p [h.port, h.use_ssl?, Net::HTTP::Get::METHOD, Net::HTTP::Head::RESPONSE_HAS_BODY, Net::HTTP.default_port, Net::HTTP::VERSION]
`)
	assert.Equals(t, out, fmt.Sprintf(`[false, "#<Net::HTTP 127.0.0.1:%s open=false>", true]
[Net::ReadTimeout, "Net::ReadTimeout with #<TCPSocket:(closed)>"]
[Errno::ECONNREFUSED, "Failed to open TCP connection to 127.0.0.1:1 (Connection refused - connect(2) for \"127.0.0.1\" port 1)"]
[SocketError, "Failed to open TCP connection to no-such-host.invalid:80 (getaddrinfo: Name or service not known)"]
[ArgumentError, "HTTP request path is empty"]
[80, false, 8080]
[80, true, "GET", false, 80, "0.6.0"]
`, strings.TrimPrefix(srv.URL, "http://127.0.0.1:")))
}

func TestNetHTTPWithoutAClientHasNoNetwork(t *testing.T) {
	out := runScript(t, `require "net/http"
begin
  Net::HTTP.get(URI("http://127.0.0.1:1/x"))
rescue SocketError => e
  puts e.message
end
p Net::HTTP::Get.new("/x").class`)
	assert.Equals(t, out, "Failed to open TCP connection to 127.0.0.1:1 (the program has no network access: its host gave it no HTTP client)\nNet::HTTP::Get\n")
}

func TestNetHTTPReachesATLSServerWithTheHostsClient(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "secure %s", r.Host)
	}))
	t.Cleanup(srv.Close)
	out := runNetScript(t, srv.Client(), srv.URL, `
u = URI(BASE)
p Net::HTTP.get(u)
Net::HTTP.start(u.host, u.port, use_ssl: true) { |http| p http.use_ssl?, http.get("/").body }
`)
	host := strings.TrimPrefix(srv.URL, "https://")
	assert.Equals(t, out, fmt.Sprintf("\"secure %s\"\ntrue\n\"secure %s\"\n", host, host))
}
