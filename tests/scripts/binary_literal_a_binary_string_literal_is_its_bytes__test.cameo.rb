# binary literal: testcase to demonstrate that a binary string literal is its
# bytes
#
# b"..." is a binary string whatever its bytes are, with the escapes of a
# double-quoted string, and b'...' with those of a single-quoted one.
#
# (Introduced in PR #417. expected output follows the __END__ separator.)
x = b"a\xFFb"
p x, x.encoding, x.bytes, b'q\xFF', b"caf\xC3\xA9".encoding, b"".encoding
p b"abc" == "abc", b"\xC3" + b"\xA9", { b"k" => 1 }[b"k"], [b"\x80", b"\x81"].map(&:bytes)
def tag(s, n) = "#{n}: #{s.bytesize}"
t = tag b"\x00", 1
puts tag(b"\x00\x01", 2), t

__END__
"a\xFFb"
#<Encoding:BINARY (ASCII-8BIT)>
[97, 255, 98]
"q\\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
true
"\xC3\xA9"
1
[[128], [129]]
2: 2
1: 1
