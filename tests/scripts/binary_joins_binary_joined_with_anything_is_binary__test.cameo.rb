# binary joins: testcase to demonstrate that binary joined with anything is
# binary
#
# Binary joined with anything is binary, whatever the parts hold: Ruby returns
# UTF-8 when the binary part is ASCII alone, and raises when both hold other
# bytes. A join of text alone stays text.
#
# (Introduced in PR #439. expected output follows the __END__ separator.)
bin = b"ab"
hi = b"\xFF"
p ("x" + bin).encoding, (bin + "x").encoding, ("é" + hi).encoding, ("é" + hi).bytes, ("x" + b"").encoding
p [bin, "é"].join("-").encoding, ["a", "é"].join.encoding, ("x".force_encoding("US-ASCII") + bin).encoding
s = "x"
s <<= bin
t = "é"
t += hi
u = "é"
u = u.prepend(bin)
p s.encoding, t.encoding, u.encoding, ("a" + "é").encoding

__END__
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
[195, 169, 255]
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
