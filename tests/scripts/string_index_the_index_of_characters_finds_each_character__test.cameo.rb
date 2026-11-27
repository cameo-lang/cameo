# string index: testcase to demonstrate that the index of characters finds
# each character
#
# A long string with characters beyond ASCII is found through an index of its
# characters. Every index gives the character chars gives, forwards,
# backwards, from the end and out of order, for several strings at once, for
# more strings than keep an index, and for slices that share bytes.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
s = "aé€😀b" * 300
chars = s.chars
n = chars.size
p n, s.bytesize
p (0...n).all? { |i| s[i] == chars[i] }
p (1..n).all? { |i| s[-i] == chars[n - i] }
p (0...n).to_a.reverse.all? { |i| s[i] == chars[i] }
p [1499, 0, 700, 64, 63, 65, 1280, 3, 1498].map { |i| s[i] == chars[i] }.all?
p s[n], s[n + 100], s[-n - 1], s[n - 1], s[-n]
others = (1..12).map { |k| "é" * (300 + k) + k.to_s }
p others.each_with_index.all? { |o, k| o[300 + k + 1] == (k + 1).to_s[0] && o[5] == "é" && s[k] == chars[k] }
head, tail = s[0, 600], s[600..]
p (0...600).all? { |i| head[i] == chars[i] }, (0...900).all? { |i| tail[i] == chars[600 + i] }, head[600], tail[900]

__END__
1500
3300
true
true
true
true
nil
nil
nil
"b"
"a"
true
true
true
nil
nil
