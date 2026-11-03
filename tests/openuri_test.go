package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The server is the net/http tests' one, with a redirect loop and a
// Last-Modified header added here.
func openURIServer(t *testing.T) *httptest.Server {
	srv := testServer(t)
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", 302) })
	mux.HandleFunc("/dated", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "identity")
		w.Write([]byte(`{"a":1}`))
	})
	return srv
}

func TestOpenURIFetchesAsRubysDoes(t *testing.T) {
	srv := openURIServer(t)
	out := runNetScript(t, srv.Client(), srv.URL, `require "open-uri"
doc = URI.open(BASE + "/hello")
p doc.class, doc.read, doc.status, doc.content_type, doc.charset, doc.base_uri.to_s == BASE + "/hello", doc.meta["x-test"], doc.metas["x-multi"], doc.content_encoding, doc.last_modified
p URI.open(BASE + "/hello") { |f| f.read.upcase }, URI.parse(BASE + "/hello").read, URI.parse(BASE + "/hello").open.status, URI.open(URI.parse(BASE + "/hello")).read.encoding
dated = URI.open(BASE + "/dated")
p dated.read, dated.read.encoding, dated.content_type, dated.charset, dated.content_encoding, dated.last_modified, URI.open(BASE + "/dated", encoding: "utf-8").read.encoding, URI.open(BASE + "/dated", "r:utf-8").read.encoding
p URI.open(BASE + "/redirect").read, URI.open(BASE + "/redirect").base_uri.to_s == BASE + "/hello", URI.open(BASE + "/redirect").status
p URI.open(BASE + "/echo", "X-Token" => "t", "Accept" => "text/plain").read, URI.open(BASE + "/echo", http_basic_authentication: ["me", "secret"]).read, URI.open(BASE + "/echo", read_timeout: 5).read
begin; URI.open(BASE + "/notfound"); rescue OpenURI::HTTPError => e; p e.class, e.message, e.io.read, e.io.status; end
begin; URI.open(BASE + "/redirect", redirect: false); rescue OpenURI::HTTPRedirect => e; p e.class, e.message, e.uri.to_s == BASE + "/hello", e.io.status; end
begin; URI.open(BASE + "/loop"); rescue RuntimeError => e; p e.message.start_with?("HTTP redirection loop: "); end
["file.txt", "ftp://example.com/x", "mailto:a@b"].each do |s|
  URI.open(s)
rescue ArgumentError => e
  puts e.message
end
begin; URI.open(BASE + "/hello", nope: 1); rescue ArgumentError => e; puts e.message; end
begin; URI.open(BASE + "/hello", "w"); rescue ArgumentError => e; puts e.message; end
begin; URI.open(BASE + "/hello", proxy: "http://p:8080"); rescue ArgumentError => e; puts e.message; end
p OpenURI::VERSION, OpenURI::HTTPRedirect.superclass, OpenURI::TooManyRedirects.superclass, OpenURI::HTTPError.superclass
`)
	assert.Equals(t, out, `OpenURI::Document
"hello, world"
["200", "OK"]
"text/plain"
"utf-8"
true
"one"
["a, b"]
[]
nil
"HELLO, WORLD"
"hello, world"
["200", "OK"]
#<Encoding:UTF-8>
"{\"a\":1}"
#<Encoding:BINARY (ASCII-8BIT)>
"application/json"
nil
["identity"]
2015-10-21 07:28:00 UTC
#<Encoding:UTF-8>
#<Encoding:UTF-8>
"hello, world"
true
["200", "OK"]
"GET /echo host=`+srv.Listener.Addr().String()+` X-Token: t body=\"\""
"GET /echo host=`+srv.Listener.Addr().String()+` Authorization: Basic bWU6c2VjcmV0 body=\"\""
"GET /echo host=`+srv.Listener.Addr().String()+` body=\"\""
OpenURI::HTTPError
"404 Not Found"
"nope\n"
["404", "Not Found"]
OpenURI::HTTPRedirect
"302 Found"
true
["302", "Found"]
true
"file.txt" is not an http or https URL: cameo reads no files
ftp://example.com/x is not an http or https URL: open-uri reaches the network alone
mailto:a@b is not an http or https URL: open-uri reaches the network alone
unrecognized option: :nope
invalid access mode w (URI::HTTP resource is read only.)
a proxy is the host's HTTP client's business: the proxy option is not supported
"0.5.0"
OpenURI::HTTPError
OpenURI::HTTPError
StandardError
`)
}
