package cameo

import (
	"testing"
)

// An Enumerator goes through a dump as the call that made it: one a
// variable holds, one in a list, one a Proc captured, one in flight as an
// argument of a call that is running, and a lazy one with its block.
func TestAnEnumeratorGoesThroughADump(t *testing.T) {
	const src = `class Countdown
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
k = 3
pairs = [1, 2, 3, 4, 5].each_slice(2)
held = [pairs, (1..3).each, 1.step(9, 4)]
evens = (1..).lazy.select(&:even?).map { |x| x * k }
down = Countdown.new(from: 3).each_with_index
sum = -> { pairs.map(&:sum) }
Test.pause
p pairs.to_a, held.map(&:to_a), evens.first(2), down.to_a, sum.call
def drain(e)
  out = e.map do |x|
    Test.pause
    x.inspect
  end
  Test.pause
  out.join(" ") + " of " + e.inspect
end
puts drain(pairs.with_index)
puts drain(evens.first(2).each)
Test.pause
p evens.class, down.size, (pairs + held[1]).to_a
`
	dump := sameAsWhole(t, src)
	everyPauseLands(t, src)
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
}

// each with a block, on an enumerator an iterating method made, runs
// that method with the block as an activation: a pause lands inside the
// block, where before it waited for the enumeration to end. The method
// is one that is a step function; an enumerator over the each of a
// script's object still runs to its end first.
func TestARunPausesInsideTheEachOfAnEnumerator(t *testing.T) {
	const src = `slices = [1, 2, 3, 4, 5].each_slice(2)
p(slices.each do |pair|
  Test.pause
  p pair
end)
p([1, 2].map.each do |x|
  Test.pause
  x * 10
end)
p(3.times.each do |i|
  Test.pause
  puts "time #{i}"
end)
p((1..3).each_with_index.each do |x, i|
  Test.pause
  puts "#{x} at #{i}"
end)
p(Set.new([1, 2]).select.each do |x|
  Test.pause
  x > 1
end)
`
	sameAsWhole(t, src)
	everyPauseLands(t, src)
}
