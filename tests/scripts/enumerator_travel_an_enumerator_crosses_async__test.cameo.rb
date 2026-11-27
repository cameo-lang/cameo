# enumerator travel: testcase to demonstrate that an enumerator crosses async
#
# A worker takes an enumerator and returns one; the enumerator of a script's
# object calls the object's method where it runs.
#
# An Enumerator travels as the call that made it: these outputs are the
# specification of what reads back.
#
# (Introduced in PR #356. expected output follows the __END__ separator.)
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

__END__
[[1, 2], [3]]
6
[[[4, 3, 2], 0], [[1], 1]]
[[1, 2], [3, 4], [5]]
[2, 4]
