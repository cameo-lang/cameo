# step args: testcase to demonstrate that a block keeps the arguments of its
# own call
#
# A builtin that calls its block step by step hands the block its arguments
# without making a slice for them: an element as the list holds it, a count as
# the builtin keeps it. What a block keeps of its arguments, a splat, is its
# own, and does not change with the next call.
#
# (Introduced in PR #389. expected output follows the __END__ separator.)
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

__END__
[0]
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
