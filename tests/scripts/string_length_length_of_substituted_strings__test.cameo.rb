# string length: testcase to demonstrate that length of substituted strings
#
# A substitution in a string that knows its characters gives a string that
# knows its own, from what matched and what replaced it, of the same length as
# in Ruby.
#
# (Introduced in PR #399. expected output follows the __END__ separator.)
a = "  Héllo Wörld\n"
made = [
  a.gsub("l", "łł"), a.gsub(/ö/) { |m| m * 3 }, a.sub("é", ""), a.gsub("z", "y"),
  a.gsub("", "-"), a.gsub(/(l+)/, '<\1é>'), a.gsub(/[éö]/, "é" => "e", "ö" => "öö"), a.sub(/\s+/, ""),
  a.gsub(/./, ""),
  a.b.gsub("l", "L").force_encoding("UTF-8"), a.gsub("é", "e").gsub("ö", "o"), "".gsub("", "é"),
]
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq

__END__
[17, 16, 13, 14, 29, 20, 15, 12, 1, 14, 14, 1]
[true]
