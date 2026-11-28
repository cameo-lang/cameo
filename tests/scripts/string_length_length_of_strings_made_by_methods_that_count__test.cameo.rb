# string length: testcase to demonstrate that length of strings made by
# methods that count
#
# A string made by a method whose result has the characters of the receiver,
# or the receiver less some of one byte, by a substitution, a join, a symbol
# or a number knows its characters, and its length is what counting them one
# by one gives, as in Ruby.
#
# (Introduced in PR #399. expected output follows the __END__ separator.)
a = "  Héllo Wörld\n"
made = [
  a.upcase, a.downcase, a.capitalize, a.swapcase, a.reverse, a.strip, a.lstrip, a.rstrip, a.chomp, a.chop,
  a.to_s, a.to_str, "".chop, "é".chop, "".strip,
  a.gsub("l", "łł"), a.gsub(/ö/) { |m| m * 3 }, a.sub("é", ""), a.gsub("z", "y"),
  [a, "é", 1, 2.5, :sým, nil, ["ü", 3]].join, [a, "é", 1].join("—"),
  [].join, ["é"].join(","), :sým.to_s, :abc.to_s, 12.to_s, 12.inspect, (2**80).to_s, 1.5.to_s,
]
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq

__END__
[14, 14, 14, 14, 14, 11, 12, 13, 13, 13, 14, 14, 0, 0, 0, 17, 16, 13, 14, 24, 18, 0, 1, 3, 3, 2, 2, 25, 3]
[true]
