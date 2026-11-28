# string index: testcase to demonstrate that an integer index returns the
# character there
#
# An Integer index reads one character without decoding the string: from the
# bytes of an ASCII, a US-ASCII or a binary string, and by walking from the
# nearer end of any other. The results are Ruby's.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
s = "héllo wörld"
p s[0], s[1], s[2], s[9], s[10], s[11], s[-1], s[-2], s[-10], s[-11], s[-12]
a = "hello"
p a[0], a[4], a[5], a[-1], a[-5], a[-6]
p format("%s", "héllo")[1], "#{:héllo}!"[-1], [1, "é"].to_s[5]
bin = b"\xFFab"
p bin[0], bin[0].encoding, bin[-1], bin[3]
us = "abc".force_encoding("US-ASCII")
p us[1], us[1].encoding, us[-4]
p :héllo[1], "héllo"[1..2], "hello"[1, 3], "hello"[3..], "héllo"[-3, 2], ""[0]

__END__
"h"
"é"
"l"
"l"
"d"
nil
"d"
"l"
"é"
"h"
nil
"h"
"o"
nil
"o"
"h"
nil
"é"
"!"
"é"
"\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
"b"
nil
"b"
#<Encoding:US-ASCII>
nil
"é"
"él"
"ell"
"lo"
"ll"
nil
