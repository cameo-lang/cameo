# enumerator travel: testcase to demonstrate that an enumerator reads back
# from the envelope
#
# An Enumerator travels as the call that made it: these outputs are the
# specification of what reads back.
#
# (Introduced in PR #356. expected output follows the __END__ separator.)
def back(e) = from_value_proto(e.to_value_proto)
k = 3
[
  [1, 2, 3, 4].each_slice(2),
  [3, 1].each_with_index,
  [1, 2].each_with_object([]),
  (1..4).reverse_each,
  (1..).lazy.map { |x| x * k }.select(&:even?).take(3),
  [1, 2, 3, 4].lazy.chunk_while { |a, b| b == a + 1 },
  [5, 6, 7].lazy.grep(6..).with_index,
  1.step(10, 3),
  (1..10) % 4,
  1.step(by: 2, to: 7),
  [1, 2].each + [3],
  Enumerator.product([1, 2], [3]),
  [1, 2, 3].to_enum(:each_slice, 2),
  [1, 2, 3].each_slice(2).with_index,
  [1, 2].cycle,
  "ab".each_char,
].each do |e|
  b = back(e)
  puts "#{b.inspect} #{b.first(4).inspect} #{b.class == e.class && b.size == e.size}"
end

__END__
#<Enumerator: [1, 2, 3, 4]:each_slice(2)> [[1, 2], [3, 4]] true
#<Enumerator: [3, 1]:each_with_index> [[3, 0], [1, 1]] true
#<Enumerator: [1, 2]:each_with_object([])> [[1, []], [2, []]] true
#<Enumerator: 1..4:reverse_each> [4, 3, 2, 1] true
#<Enumerator::Lazy: #<Enumerator::Lazy: #<Enumerator::Lazy: #<Enumerator::Lazy: 1..>:map>:select>:take(3)> [6, 12, 18] true
#<Enumerator::Lazy: #<Enumerator::Lazy: [1, 2, 3, 4]>:chunk_while> [[1, 2, 3, 4]] true
#<Enumerator::Lazy: #<Enumerator::Lazy: #<Enumerator::Lazy: [5, 6, 7]>:grep(6..)>:with_index> [[6, 0], [7, 1]] true
(1.step(10, 3)) [1, 4, 7, 10] true
((1..10).%(4)) [1, 5, 9] true
(1.step(by: 2, to: 7)) [1, 3, 5, 7] true
#<Enumerator::Chain: [#<Enumerator: [1, 2]:each>, [3]]> [1, 2, 3] true
#<Enumerator::Product: [[1, 2], [3]]> [[1, 3], [2, 3]] true
#<Enumerator: [1, 2, 3]:each_slice(2)> [[1, 2], [3]] true
#<Enumerator: #<Enumerator: [1, 2, 3]:each_slice(2)>:with_index> [[[1, 2], 0], [[3], 1]] true
#<Enumerator: [1, 2]:cycle> [1, 2, 1, 2] true
#<Enumerator: "ab":each_char> ["a", "b"] true
