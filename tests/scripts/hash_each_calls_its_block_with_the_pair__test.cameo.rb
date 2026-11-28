# hash each: testcase to demonstrate that hash each calls its block with the
# pair
#
# A Hash's each and each_pair call the block with one value, the pair of the
# key and the value, as Ruby's do. A block with a parameter for each takes
# them apart; one with a single parameter, a splat, or a method given as the
# block gets the pair.
#
# (Introduced in PR #390. expected output follows the __END__ separator.)
h = { a: 1, b: 2 }
h.each { |k, v| p [k, v] }
{ a: 1 }.each { |k, v| p k, v }
h.each { |pair| p pair }
h.each_pair { |pair| p pair }
h.each { |*all| p all }
h.each { |k, v, more| p [k, v, more] }
h.each { |k, v = 5| p [k, v] }
h.each { |(k, v)| p [v, k] }
p h.each(&:first)
p h.each { }.size
h.each_with_index { |pair, i| p [pair, i] }
h.each_with_index { |(k, v), i| p [k, v, i] }
p h.map { |pair| pair }, h.map { |k, v| [v, k] }
p h.select { |k| k == :a }, h.select { |k, v| v == 2 }

__END__
[:a, 1]
[:b, 2]
:a
1
[:a, 1]
[:b, 2]
[:a, 1]
[:b, 2]
[[:a, 1]]
[[:b, 2]]
[:a, 1, nil]
[:b, 2, nil]
[:a, 1]
[:b, 2]
[1, :a]
[2, :b]
{a: 1, b: 2}
2
[[:a, 1], 0]
[[:b, 2], 1]
[:a, 1, 0]
[:b, 2, 1]
[[:a, 1], [:b, 2]]
[[1, :a], [2, :b]]
{a: 1}
{b: 2}
