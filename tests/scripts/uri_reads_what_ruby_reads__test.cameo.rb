# uri: testcase to demonstrate that URI reads what Ruby reads
#
# This test holds the forms the review of the library found refused, hung on
# or read otherwise than Ruby 3.4.1 does: a query with a bare % or one hex
# digit, a bad escape's message, decoding that keeps only the bad escapes, an
# empty separator, the Array form of build and the class's own scheme, and an
# FTP path's %2F and first ;type=.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
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

__END__
"a=100%"
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
