# tail call block: testcase to demonstrate that a block breaks out of a method
# that ends in a call
#
# A method that was given a block makes no tail call, so a break out of the
# block ends the method, as in Ruby. new returns the instance when initialize
# ends in a tail call of a method that returns early.
#
# (Introduced in PR #382. expected output follows the __END__ separator.)
def helper(blk) = blk.call(1)
def each_thing(&blk)
  helper(blk)
end
p(each_thing { |x| break x + 4 })
class Q
  int :v
  def initialize(v)
    self.v = v
    check(v)
  end
  def check(v)
    return :bad if v < 0
    :ok
  end
end
p Q.new(-1).v

__END__
5
-1
