# interpolation: testcase to demonstrate that a string with interpolation is
# its parts
#
# A string with interpolation is its parts, whether a part is a string with
# parts of its own, an object with a to_s, or left by next; a string it is
# made of is unchanged.
#
# (Introduced in PR #393. expected output follows the __END__ separator.)
x = "é".b
r = begin
  "ü#{x}"
rescue => e
  e.class
end
p r
p "#{}", "#{"only"}", "a#{"b#{"c#{1}d"}e"}f#{2}", "#{"x" * 40}#{"y" * 40}#{"z" * 40}".size
p [1, 2, 3].map { |i| "v#{i}-#{i * 2}" }
p [1, 2].map { |i| "a#{next "n" if i == 1}b#{i}" }
class Pt
  def to_s = "pt"
end
p "#{Pt.new}|#{Pt.new}", "#{1}#{Pt.new}#{2}#{nil}#{:s}#{1.5}"
s = "kept"
t = "#{s}"
u = "#{s}!"
p s, t, u, "#{t}#{u}"

__END__
Encoding::CompatibilityError
""
"only"
"abc1def2"
120
["v1-2", "v2-4", "v3-6"]
["n", "ab2"]
"pt|pt"
"1pt2s1.5"
"kept"
"kept"
"kept!"
"keptkept!"
