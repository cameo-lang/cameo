package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The scripts are plain Ruby, and the expected outputs are Ruby 3.4's,
// with uri 1.0.2.

func TestURIParsesEveryKindOfReference(t *testing.T) {
	out := runScript(t, `require "uri"
p URI.split("mailto:joe@example.com?subject=hi")
p URI.split("http://"), URI.split("http:"), URI.split(""), URI.split("//h/p"), URI.split("?q")
p URI("http://h?q").path, URI("http://").host, URI("urn:isbn:123").opaque, URI("x.com/p").path
p URI("http://u@x.com").password, URI("http://u:@x.com").password, URI("http://:p@x.com").user
p URI("http://x.com:/").port, URI("http://x.com:0").to_s, URI("http://x.com:65536").port
`)
	assert.Equals(t, out, `["mailto", nil, nil, nil, nil, nil, "joe@example.com?subject=hi", nil, nil]
["http", nil, "", nil, nil, "", nil, nil, nil]
["http", nil, nil, nil, nil, "", nil, nil, nil]
[nil, nil, nil, nil, nil, "", nil, nil, nil]
[nil, nil, "h", nil, nil, "/p", nil, nil, nil]
[nil, nil, nil, nil, nil, "", nil, "q", nil]
""
""
"isbn:123"
"x.com/p"
nil
""
""
80
"http://x.com:0"
65536
`)
}

func TestURIRefusesWhatRubyRefuses(t *testing.T) {
	out := runScript(t, `require "uri"
["http://x.com:-1", "ht tp://x", "http://x.com/€"].each do |s|
  URI.parse(s)
rescue URI::InvalidURIError => e
  puts e.message
end
begin; URI.parse(nil); rescue URI::InvalidURIError => e; puts e.message; end
begin; URI(nil); rescue ArgumentError => e; puts e.message; end
begin; URI.join("x", "y"); rescue URI::BadURIError => e; puts e.message; end
begin; URI::HTTP.build(host: "x com", path: "/p"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI::HTTP.build(host: "x.com", path: "p"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI::HTTP.build(host: "a b"); rescue URI::InvalidComponentError => e; puts e.message; end
begin; URI("http://x.com/a").select(:nope); rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `bad URI (is not URI?): "http://x.com:-1"
bad URI (is not URI?): "ht tp://x"
URI must be ascii only "http://x.com/\u20AC"
bad URI (is not URI?): nil
bad argument (expected URI object or URI string)
both URI are relative
bad component(expected host component): x com
bad component(expected absolute path component): p
bad component(expected host component): a b
expected of components of URI::HTTP (scheme, userinfo, host, port, path, query, fragment)
`)
}

// A URI has no setters. Ruby changes the URI in place; cameo refuses the
// assignment, with itself, and the FTP and MailTo writers, and a different
// URI is parsed, joined, merged or built. The last line prints what Ruby
// prints.
func TestURIHasNoSetters(t *testing.T) {
	out := runScript(t, `require "uri"
a = URI("http://x.com/a")
b = a
begin; a.path = "/b"; rescue NoMethodError => e; puts e.message; end
begin; a.with(query: "q"); rescue NoMethodError => e; puts e.message; end
begin; a.with { |x| { port: 80 } }; rescue NoMethodError => e; puts e.message; end
f = URI("ftp://h/pub/f;type=a")
begin; f.typecode = "i"; rescue NoMethodError => e; puts e.message; end
m = URI("mailto:a@b.c")
begin; m.to = "x@y.z"; rescue NoMethodError => e; puts e.message; end
begin; m.headers = [["subject", "hi"]]; rescue NoMethodError => e; puts e.message; end
p a == b, a + "/b", a.merge("?q=1"), URI.join(a, "c#f"), URI::HTTP.build(host: "x.com", port: 444, path: "/b", query: "q=1")
`)
	assert.Equals(t, out, `undefined method 'path=' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'query=' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'with' for an instance of URI::HTTP: a URI is parsed, joined, merged or built, not changed
undefined method 'typecode=' for an instance of URI::FTP: a URI is parsed, joined, merged or built, not changed
undefined method 'to=' for an instance of URI::MailTo: a URI is parsed, joined, merged or built, not changed
undefined method 'headers=' for an instance of URI::MailTo: a URI is parsed, joined, merged or built, not changed
true
#<URI::HTTP http://x.com/b>
#<URI::HTTP http://x.com/a?q=1>
#<URI::HTTP http://x.com/c#f>
#<URI::HTTP http://x.com:444/b?q=1>
`)
}

func TestURIMergeAndRouteFollowRuby(t *testing.T) {
	out := runScript(t, `require "uri"
b = URI("http://x.com/a/b")
puts b.merge("//y.com/b"), b.merge("?q"), b.merge("#f"), b.merge(""), b.merge("c"), b.merge("/c"), b.merge("other:x")
puts URI("http://x.com/a/b/").merge(".."), URI("http://x.com/a/b?q").merge("c"), URI("http://x.com").merge("c")
[["http://x.com/a/b", "http://x.com/a/b"], ["http://x.com/a/b", "http://x.com/a/"], ["http://x.com/a/b", "http://x.com/c"],
 ["http://x.com/a/b", "http://y.com/c"], ["http://x.com/a/b", "https://x.com/c"], ["http://x.com/a/b", "http://x.com/a/b?q"],
 ["http://x.com/a/b/", "http://x.com/"], ["http://x.com/a", "http://x.com/a/"], ["http://x.com", "http://x.com/a"],
 ["http://x.com/a", "http://x.com"]].each do |from, to|
  p URI(from).route_to(to).to_s
end
p URI("http://x.com/a/b") - "http://x.com/a/"
p URI.join("http://a/b/", "//other.com/x", "y"), URI.join("http://a/b/", "mailto:z@x", "c")
`)
	assert.Equals(t, out, `http://y.com/b
http://x.com/a/b?q
http://x.com/a/b#f
http://x.com/a/b
http://x.com/a/c
http://x.com/c
other:x
http://x.com/a/
http://x.com/a/c
http://x.com/c
""
"./"
"../c"
"//y.com/c"
"https://x.com/c"
"?q"
"../../"
"a/"
"/a"
"../"
#<URI::Generic b>
#<URI::HTTP http://other.com/y>
#<URI::MailTo mailto:z@x>
`)
}

func TestURIFormsEncodeAndDecodeAsRuby(t *testing.T) {
	out := runScript(t, `require "uri"
p URI.encode_www_form([["q", "ruby lang"], ["lang", "en"]]), URI.encode_www_form("a" => "b", "c" => ["d", nil], f: true, h: 1.5)
p URI.decode_www_form("a=1;b=2"), URI.decode_www_form("a=1;b=2", separator: ";"), URI.decode_www_form("")
p URI.decode_www_form("&&a=1&&"), URI.decode_www_form("a=%ZZ"), URI.decode_www_form("=1"), URI.decode_www_form("a==1")
p URI.encode_www_form_component("a\nb\tc\u0000"), URI.encode_www_form_component(123), URI.encode_www_form_component(nil)
p URI.decode_www_form_component("%E2%82%AC"), URI.decode_www_form_component("%E2%82").encoding
begin; URI.decode_www_form_component("abc%"); rescue ArgumentError => e; puts e.message; end
p URI.extract("see http://a.com/x and ftp://b", ["http"])
URI.extract("a http://x.org b") { |s| puts s }
`)
	assert.Equals(t, out, `"q=ruby+lang&lang=en"
"a=b&c=d&&f=true&h=1.5"
[["a", "1;b=2"]]
[["a", "1"], ["b", "2"]]
[]
[["", ""], ["", ""], ["a", "1"], ["", ""]]
[["a", "%ZZ"]]
[["", "1"]]
[["a", "=1"]]
"a%0Ab%09c%00"
"123"
""
"€"
#<Encoding:BINARY (ASCII-8BIT)>
invalid %-encoding (abc%)
["http://a.com/x"]
http://x.org
`)
}

func TestURIMailToAndLDAPComponents(t *testing.T) {
	out := runScript(t, `require "uri"
m = URI("mailto:a@b.c,d@e.f?subject=hi&body=x")
p m.to, m.headers, m.select(:to, :headers), m.to_mailtext
p URI::MailTo.build(to: "joe@example.com", headers: [["subject", "hi"]]).to_s
l = URI("ldap://x/dc=ex?a,b?sub?(cn=x)")
p l.class, l.dn, l.attributes, l.scope, l.filter, l.extensions, l.port
p URI::Generic.new("http", nil, "x.com", 80, nil, "/p", nil, "q", "f").to_s
p URI.for("Http", nil, "x.com", nil, nil, "/p", nil, nil, nil).class, URI::Generic.ancestors.take(3)
`)
	assert.Equals(t, out, `"a@b.c,d@e.f"
[["subject", "hi"], ["body", "x"]]
["a@b.c,d@e.f", [["subject", "hi"], ["body", "x"]]]
"To: a@b.c,d@e.f\nSubject: hi\n\nx\n"
"mailto:joe@example.com?subject=hi"
URI::LDAP
"dc=ex"
"a,b"
"sub"
"(cn=x)"
nil
389
"http://x.com:80/p?q#f"
URI::HTTP
[URI::Generic, URI, Object]
`)
}

// TestURIReadsWhatRubyReads holds the forms the review of the library
// found refused, hung on or read otherwise than Ruby 3.4.1 does: a query
// with a bare % or one hex digit, a bad escape's message, decoding that
// keeps only the bad escapes, an empty separator, the Array form of build
// and the class's own scheme, and an FTP path's %2F and first ;type=.
func TestURIReadsWhatRubyReads(t *testing.T) {
	out := runScript(t, `require "uri"
p URI("http://x.com/?a=100%").query, URI("http://x.com/?p=5%2").query
begin; URI("http://x.com/?x=%GG"); rescue URI::InvalidURIError => e; puts e.message; end
p URI.decode_www_form("q=50%+off&r=%ZZ%41"), URI.decode_www_form("a=1&b=2", separator: "")
p URI::HTTP.build([nil, "www.example.com", nil, "/path", "query", "fragment"]).to_s, URI::HTTP.build(scheme: "ftp", host: "x").to_s
u = URI("ftp://h/%2Fetc/x;type=a;type=i"); p u.path, u.typecode
begin; URI::HTTP.build([nil, "h"]); rescue ArgumentError => e; puts e.message; end
begin; URI("http://x.com:99999999999999999999"); rescue URI::InvalidComponentError => e; puts e.message; end
class Box
  int :x
  def initialize(x) = self.x = x
  def with(**kw) = "mine #{kw}"
end
p Box.new(1).with(x: 2)
`)
	assert.Equals(t, out, `"a=100%"
"p=5%2"
invalid percent escape: %GG
[["q", "50% off"], ["r", "%ZZA"]]
[["a", "1&b=2"]]
"http://www.example.com/path?query#fragment"
"http://x"
"/etc/x"
"a;type=i"
expected Array of or Hash of components of URI::HTTP (userinfo, host, port, path, query, fragment)
bad component(expected port component): "99999999999999999999"
"mine {x: 2}"
`)
}
