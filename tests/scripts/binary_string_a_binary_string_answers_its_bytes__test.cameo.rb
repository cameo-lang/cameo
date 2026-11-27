# binary string: testcase to demonstrate that a binary string answers its
# bytes
#
# A binary string answers what reads, makes and compares bytes, and a pattern,
# as Ruby's does.
#
# (Introduced in PR #403. expected output follows the __END__ separator.)
b = b"héllo\xFF"
p b.length, b.size, b.bytesize, b[1], b[1, 2], b[-1], b.bytes.first(3), b.getbyte(0), b.empty?
p b + "x".b, b * 2 == b + b, b == b"héllo\xFF", b.index("l"), b.include?("ll"), b.start_with?("h"), b.end_with?(b"\xFF")
p b.delete_prefix("h"), b.unpack("C2"), b.encoding, b.ascii_only?, b.hash == b"héllo\xFF".hash, b.dup == b
p b.sub("l", "L"), b.gsub(/l/n, "L"), b.scan(/l/n).size, b =~ /l/n, b.match?(/\xFF/n)

__END__
7
7
7
"\xC3"
"\xC3\xA9"
"\xFF"
[104, 195, 169]
104
false
"h\xC3\xA9llo\xFFx"
true
true
3
true
true
true
"\xC3\xA9llo\xFF"
[104, 195]
#<Encoding:BINARY (ASCII-8BIT)>
false
true
true
"h\xC3\xA9Llo\xFF"
"h\xC3\xA9LLo\xFF"
2
3
true
