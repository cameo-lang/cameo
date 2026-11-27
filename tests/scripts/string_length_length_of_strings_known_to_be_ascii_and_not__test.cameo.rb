# string length: testcase to demonstrate that length of strings known to be
# ASCII and not
#
# A string's length is its characters, whether the string is known to be
# ASCII, a literal, a repetition or a sum of such, or is not.
#
# (Introduced in PR #398. expected output follows the __END__ separator.)
p "abc".length, ("ab" * 3).size, ("é" * 3).length, ("a" + "é").length, ("a" + "b").length
p "".length, ("" * 5).length, "é".b.length, ("x" * 3 + "é").size, ("é" + "x" * 3).size
p ("a" * 2).force_encoding("ASCII-8BIT").length, "añb".size, ("a" * 3).encoding, ("a" + "b").encoding
s = "ab" * 2
p s == "abab", s.eql?("abab"), { s => 1 }["abab"], s.hash == "abab".hash

__END__
3
6
3
2
2
0
0
2
4
4
2
3
#<Encoding:UTF-8>
#<Encoding:UTF-8>
true
true
1
true
