# binary paths: testcase to demonstrate that substitution takes no bytes from
# a block or a hash
#
# What a block or a hash gives sub and gsub to put in is checked as an
# argument is: bytes that are no characters do not go into text, so no UTF-8
# String is made of them. ASCII bytes are text.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
def try
  yield
rescue Encoding::CompatibilityError => e
  puts e.message
end
try { "abc".gsub("b") { b"\xFF" } }
try { "abc".gsub("b", { "b" => b"\xFF" }) }
try { "abc".sub(/b/) { b"\xFF" } }
s = "abc".gsub("b") { b"x" }
p s, s.encoding, s.valid_encoding?, s.length
r = b"a\xFFc".gsub(b"\xFF") { b"\xFE" }
p r, r.encoding

__END__
incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
"axc"
#<Encoding:UTF-8>
true
3
"a\xFEc"
#<Encoding:BINARY (ASCII-8BIT)>
