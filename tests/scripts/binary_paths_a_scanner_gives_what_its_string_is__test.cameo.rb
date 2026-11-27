# binary paths: testcase to demonstrate that a scanner gives what its string
# is
#
# A StringScanner over a binary String gives bytes, and one over text gives
# text.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
require "strscan"
sc = StringScanner.new(b"\xFFab")
p sc.scan(/./n), sc.peek(1), sc.rest, sc.string, sc.getch, sc.pre_match, sc.post_match
p [sc.rest, sc.string, sc.matched].map(&:encoding).uniq
tx = StringScanner.new("héllo")
p tx.scan(/h./), tx.rest, tx.rest.encoding
tx.string = b"\xFEz"
p tx.scan(/./n), tx.rest.encoding

__END__
"\xFF"
"a"
"ab"
"\xFFab"
"a"
"\xFF"
"b"
[#<Encoding:BINARY (ASCII-8BIT)>]
"hé"
"llo"
#<Encoding:UTF-8>
"\xFE"
#<Encoding:BINARY (ASCII-8BIT)>
