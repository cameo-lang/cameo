# uri: testcase to demonstrate that URI forms encode and decode as Ruby
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# uri 1.0.2.
#
# (Introduced in PR #263. expected output follows the __END__ separator.)
require "uri"
p URI.encode_www_form([["q", "ruby lang"], ["lang", "en"]]), URI.encode_www_form("a" => "b", "c" => ["d", nil], f: true, h: 1.5)
p URI.decode_www_form("a=1;b=2"), URI.decode_www_form("a=1;b=2", separator: ";"), URI.decode_www_form("")
p URI.decode_www_form("&&a=1&&"), URI.decode_www_form("a=%ZZ"), URI.decode_www_form("=1"), URI.decode_www_form("a==1")
p URI.encode_www_form_component("a\nb\tc\u0000"), URI.encode_www_form_component(123), URI.encode_www_form_component(nil)
p URI.decode_www_form_component("%E2%82%AC")
begin; URI.decode_www_form_component("%E2%82"); rescue Encoding::InvalidByteSequenceError => e; puts e.message; end
begin; URI.decode_www_form("a=%FF"); rescue Encoding::InvalidByteSequenceError => e; puts e.class; end
begin; URI.decode_www_form_component("abc%"); rescue ArgumentError => e; puts e.message; end
p URI.extract("see http://a.com/x and ftp://b", ["http"])
URI.extract("a http://x.org b") { |s| puts s }

__END__
"q=ruby+lang&lang=en"
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
invalid byte sequence in UTF-8: URI returns text, and these bytes are none (see https://15b.it/inv)
Encoding::InvalidByteSequenceError
invalid %-encoding (abc%)
["http://a.com/x"]
http://x.org
