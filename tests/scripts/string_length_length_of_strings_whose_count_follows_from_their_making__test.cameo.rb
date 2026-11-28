# string length: testcase to demonstrate that length of strings whose count
# follows from their making
#
# tr and squeeze count the characters they write, padding and the removal of a
# prefix or a suffix have theirs from the receiver and the argument, and what
# split, scan and CSV cut out of ASCII is ASCII: each length is what counting
# one by one gives, as in Ruby.
#
# (Introduced in PR #399. expected output follows the __END__ separator.)
require "csv"
a = "  Héllo Wörld\n"
x = "hello, world  foo"
made = [
  a.tr("lo", "łø"), a.tr("l", ""), a.tr("^l", "é"), a.tr("a-z", "A-Z"), x.tr("o", "ö"),
  a.squeeze, "aaééé  b".squeeze, "".squeeze, x.squeeze,
  a.center(20), a.center(3), a.ljust(20, "é"), a.rjust(21, "éø"), x.center(30, "*"), "".rjust(3, "é"),
  a.chomp("\n"), a.chomp("ld\n"), a.chomp("x"), a.chomp(""), "é\r\n\n".chomp(""), x.chomp("foo"),
  a.delete_prefix("  H"), a.delete_prefix("x"), a.delete_suffix("rld\n"), "éa".delete_prefix("é"),
  "é".delete_prefix("é"), "é".delete_suffix("é"),
]
made += a.split + a.split("l") + x.split(", ") + x.split("") + a.split("") + x.split(/o/, 2) + a.split(/(ö)/)
made += x.scan(/o./) + a.scan(/[éö]./) + x.scan(/(l)(.)/).flatten + a.scan(/(l)(.)/).flatten + x.scan("o")
made += CSV.parse("a,b c,\"d,e\"\n1,,3\n").flatten.compact + CSV.parse("é,b\nc,ö\n").flatten + CSV.parse_line("x,y")
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq

__END__
[14, 11, 14, 14, 17, 12, 4, 0, 14, 20, 14, 20, 21, 30, 3, 13, 11, 14, 13, 1, 14, 11, 14, 10, 1, 0, 0, 5, 5, 4, 0, 5, 2, 5, 10, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 4, 12, 9, 1, 4, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 3, 1, 1, 1, 1, 1, 1, 1, 1]
[true]
