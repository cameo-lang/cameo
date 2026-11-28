# binary paths: testcase to demonstrate that a match in a binary string is
# bytes
#
# What a pattern finds in a binary String is bytes: the parts of a match, of
# partition and rpartition, what scan returns, and a MatchData's own methods.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
x = b"\xFFab"
m = x.match(/a/)
p m.pre_match, m.post_match, m[0], m.string, m.to_a
p [m.pre_match, m.post_match, m[0], m.string].map(&:encoding).uniq
p x.partition("a"), x.partition(/a/), x.rpartition("a"), x.rpartition(/a/)
p x.partition("a").map(&:encoding).uniq, x.scan(/./n), x.scan(/./n).map(&:encoding).uniq
p x.match(/(?<x>\xFF)/n).named_captures, x.match(/(a)(b)/).captures
p(/a/.match(x).pre_match, x =~ /b/)
t = "héllo".match(/l+/)
p t.pre_match, t.post_match, t[0], t.pre_match.encoding

__END__
"\xFF"
"b"
"a"
"\xFFab"
["a"]
[#<Encoding:BINARY (ASCII-8BIT)>]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
[#<Encoding:BINARY (ASCII-8BIT)>]
["\xFF", "a", "b"]
[#<Encoding:BINARY (ASCII-8BIT)>]
{"x" => "\xFF"}
["a", "b"]
"\xFF"
2
"hé"
"o"
"ll"
#<Encoding:UTF-8>
