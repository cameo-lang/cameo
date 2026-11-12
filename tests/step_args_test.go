package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A builtin that calls its block step by step hands the block its
// arguments without making a slice for them: an element as the list
// holds it, a count as the builtin keeps it. What a block keeps of its
// arguments, a splat, is its own, and does not change with the next
// call.
func TestABlockKeepsTheArgumentsOfItsOwnCall(t *testing.T) {
	out := runScript(t, `
3.times { |*a| puts a.inspect }
[[1, 2], [3, 4]].each { |a, b| puts a + b }
1.upto(3) { |i| puts i }
3.downto(1) { |i, j| puts [i, j].inspect }
(1..3).each(&:to_s)
puts [5, 6].map(&:to_s).inspect
puts [1, 2, 3].map { |*a| a }.inspect
puts (1..4).each { |i| break i * 10 if i == 3 }
puts 4.times { |i| next if i.odd?; puts i }
{ a: 1, b: 2 }.each { |k, v| puts "#{k}=#{v}" }
puts [1, 2, 3].each_with_index.map { |x, i| x * i }.inspect
lists = [1, 2, 3].map { |x| [x].map { |*a| a } }
puts lists.inspect
`)
	assert.Equals(t, out, `[0]
[1]
[2]
3
7
1
2
3
[3, nil]
[2, nil]
[1, nil]
["5", "6"]
[[1], [2], [3]]
30
0
2
4
a=1
b=2
[0, 2, 6]
[[[1]], [[2]], [[3]]]
`)
}

// The evaluator does the steps of a walk and of a count itself, from
// the second on, with the index or the Integer as a number of its own.
// What the builtins return, where they stop, and what they call the
// block with is as when the stepper does every step: this output is
// the one from before the evaluator did.
func TestWalksAndCountsTheEvaluatorSteps(t *testing.T) {
	out := runScript(t, `
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
`)
	assert.Equals(t, out, `4
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
:a
:b
:c
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
`)
}
