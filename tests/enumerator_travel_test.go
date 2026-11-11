package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// An Enumerator travels as the call that made it: these outputs are the
// specification of what reads back.

func TestAnEnumeratorReadsBackFromTheEnvelope(t *testing.T) {
	out := runScript(t, `
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
`)
	assert.Equals(t, out, `#<Enumerator: [1, 2, 3, 4]:each_slice(2)> [[1, 2], [3, 4]] true
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
`)
}

func TestAnEnumeratorCrossesAsync(t *testing.T) {
	// A worker takes an enumerator and returns one; the enumerator of a
	// script's object calls the object's method where it runs.
	out := runScript(t, `
include Cameo
class Countdown
  include Enumerable
  int :from
  def each
    n = from
    while n > 0
      yield n
      n -= 1
    end
  end
end
class W
  def list(e) = e.to_a
  def total(e) = e.sum
  def pairs(n) = (1..n).each_slice(2)
  def evens = (1..).lazy.select(&:even?)
end
p await async(W.new, :list, [1, 2, 3].each_slice(2))
p await async(W.new, :total, Countdown.new(from: 3).to_enum)
p await async(W.new, :list, Countdown.new(from: 4).each_slice(3).with_index)
p (await async(W.new, :pairs, 5)).to_a
p (await async(W.new, :evens)).first(2)
`)
	assert.Equals(t, out, "[[1, 2], [3]]\n6\n[[[4, 3, 2], 0], [[1], 1]]\n[[1, 2], [3, 4], [5]]\n[2, 4]\n")
}

func TestAnEnumeratorThatCannotTravelSaysWhy(t *testing.T) {
	out := runScript(t, `
require "stringio"
dice = Random.new(1)
[
  -> { StringIO.new("a\nb").each_line },
  -> { [dice].each },
  -> { [1, 2].each_with_object(dice) },
  -> { [1, 2].lazy.map { dice.rand } },
  -> { [1].each + [dice] },
].each do |f|
  f.call.to_value_proto
rescue TypeError => e
  puts e.message
end
`)
	assert.Equals(t, out, `an Enumerator cannot be serialised
an Enumerator of a Random cannot be serialised
an Enumerator with the argument a Random cannot be serialised
an Enumerator with a block, a Proc that captured dice, a Random cannot be serialised
an Enumerator that combines a Random cannot be serialised
`)
}
