# binary paths: testcase to demonstrate that a binary string is itself as a
# string
#
# to_s, to_str and String() of a binary String are the String, in its
# encoding.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
x = b"\xFFab"
p x.to_s, x.to_str, String(x), [x].map(&:to_s)
p x.to_s.encoding, String(x).encoding, String("é").encoding, String(12), String(:a)

__END__
"\xFFab"
"\xFFab"
"\xFFab"
["\xFFab"]
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
"12"
"a"
