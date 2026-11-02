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
v = URI("http://x.com/a")
begin; v.scheme = "1x"; rescue URI::InvalidComponentError => e; puts e.message; end
begin; v.host = "a b"; rescue URI::InvalidComponentError => e; puts e.message; end
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
bad component(expected scheme component): 1x
bad component(expected host component): a b
expected of components of URI::HTTP (scheme, userinfo, host, port, path, query, fragment)
`)
}

// Where Ruby changes the URI in place, so that b would change with a,
// cameo rebinds a to the copy and b keeps the URI as it was; with is
// cameo's. Every other line prints what Ruby prints.
func TestURISettersReturnACopyTheVariableIsReboundTo(t *testing.T) {
	out := runScript(t, `require "uri"
a = URI("http://x.com/a")
b = a
a.path = "/b"
a.query = "a b#"
a.fragment = "f g"
a.port = nil
a.hostname = "::1"
p a, b, a.port, a.hostname, a.with(path: "/c")
c = URI("http://x.com/x")
c.port = 80
p c.to_s, c.port
w = URI("http://x.com")
w.userinfo = "a:b"
w.password = "z"
p w, w.user, w.password
f = URI("ftp://h/pub/f")
p f.path, f.to_s
f.typecode = "a"
p f.to_s, f.typecode
`)
	assert.Equals(t, out, `#<URI::HTTP http://[::1]/b?a%20b%23#f%20g>
#<URI::HTTP http://x.com/a>
nil
"::1"
#<URI::HTTP http://[::1]/c?a%20b%23#f%20g>
"http://x.com/x"
80
#<URI::HTTP http://a:z@x.com>
"a"
"z"
"pub/f"
"ftp://h/pub/f"
"ftp://h/pub/f;type=a"
"a"
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
p URI.decode_www_form_component("%E2%82%AC"), URI.decode_www_form_component("%E2%82").valid_encoding?
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
false
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
