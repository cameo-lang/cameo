# string length: testcase to demonstrate that length of strings made of
# counted strings
#
# A string made of strings that know their characters knows its own: a sum, a
# repetition, an interpolation, a part of a string, a character, a number's
# text. One whose bytes were relabelled is counted, and has the same length as
# in Ruby.
#
# (Introduced in PR #398. expected output follows the __END__ separator.)
a = "héllo"
b = "wörld"
n = 12
p (a + b).length, (a * 3).length, "#{a} #{b}".length, "#{a}#{n}#{1.5}#{:sým}#{nil}".length
p a[1, 3].length, a[1..].length, a[1].length, a[-1].length, a.chars.map(&:length), a.chars.join.length
p n.to_s.length, 1.5.to_s.length, (a + n.to_s).length, "#{a}".length, "#{n}".length
p a.b.length, a.b.force_encoding("UTF-8").length, (a.b + "x").length
p "#{a.b.force_encoding("UTF-8")}x".length, a.dup.force_encoding("ASCII-8BIT").length

__END__
10
15
11
13
3
4
1
1
[1, 1, 1, 1, 1]
5
2
3
7
5
2
6
5
7
6
6
