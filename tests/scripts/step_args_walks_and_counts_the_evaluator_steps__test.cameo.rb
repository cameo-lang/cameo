# step args: testcase to demonstrate that walks and counts the evaluator steps
#
# The evaluator does the steps of a walk and of a count itself, from the
# second on, with the index or the Integer as a number of its own. What the
# builtins return, where they stop, and what they call the block with is as
# when the stepper does every step.
#
# (Introduced in PR #389. expected output follows the __END__ separator.)
p (1..).each { |i| break i if i > 3 }
p 9223372036854775806.upto(9223372036854775809) { |i| p i }
p 3.downto(1) { |i| p i }
p (9223372036854775805..9223372036854775808).each { |i| p i }
p (1...4).each { |i| p i }
p 0.times { |i| p i }
p [1, 2, 3, 4].drop_while { |x| x < 3 }
p [1, 2, 3, 4].take_while { |x| x < 3 }
p [1, 2, 3].each_with_index { |x, i| p [x, i] }
p [1, 2, 3].each_with_object([]) { |x, m| m + [x] }
p [1, 2, 3].inject { |a, b| a + b }
p [1, 2, 3].sum { |x| x * 2 }
p [1, 2, 3].count { |x| x.odd? }
p [1, 2, 3].partition { |x| x.odd? }
p [1, 2, 3].find { |x| x > 1 }
p [1, 2, 3].filter_map { |x| x * 2 if x.odd? }
h = { a: 1, b: 2, c: 3 }
p h.select { |k, v| v > 1 }
p h.each_key { |k| p k }.size
p h.transform_values { |v| v * 2 }
p h.each { |pair| p pair }.size
p h.map { |k, v| [k, v * 2] }
p [3, 4].each_index { |i| p i }
p (1..3).map { |x| x * 2 }
p [1, 2, 3].each { |x| next if x == 2; p x }
p [1, 2, 3].map { |x| next 0 if x == 2; x }
r = [1, 2, 3].each do |x|
  begin
    raise "no" if x == 2
  rescue => e
    p e.message
  end
end
p r
p 5.times.map { |i| i * i }
p 1.step(10, 3) { |i| p i }

__END__
4
9223372036854775806
9223372036854775807
9223372036854775808
9223372036854775809
9223372036854775806
3
2
1
3
9223372036854775805
9223372036854775806
9223372036854775807
9223372036854775808
9223372036854775805..9223372036854775808
1
2
3
1...4
0
[3, 4]
[1, 2]
[1, 0]
[2, 1]
[3, 2]
[1, 2, 3]
[1, 2, 3]
6
12
2
[[1, 3], [2]]
2
[2, 6]
{b: 2, c: 3}
:a
:b
:c
3
{a: 2, b: 4, c: 6}
[:a, 1]
[:b, 2]
[:c, 3]
3
[[:a, 2], [:b, 4], [:c, 6]]
0
1
[3, 4]
[2, 4, 6]
1
3
[1, 2, 3]
[1, 0, 3]
"no"
[1, 2, 3]
[0, 1, 4, 9, 16]
1
4
7
10
1
