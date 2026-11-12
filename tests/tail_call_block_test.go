package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A method that was given a block makes no tail call, so a break out of
// the block ends the method, as in Ruby. new returns the instance when
// initialize ends in a tail call of a method that returns early.
func TestABlockBreaksOutOfAMethodThatEndsInACall(t *testing.T) {
	out := runScript(t, `def helper(blk) = blk.call(1)
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
`)
	assert.Equals(t, out, "5\n-1\n")
}
