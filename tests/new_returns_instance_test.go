package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// new returns the instance when initialize is left by a return, with a
// value or without one, from a loop or from under an ensure.
func TestNewReturnsTheInstanceWhenInitializeReturnsEarly(t *testing.T) {
	out := runScript(t, `class A
  int :n
  def initialize(n)
    self.n = n
    return if n > 0
    self.n = -1
  end
end
class B
  int :n
  def initialize(n)
    self.n = n
    return 42
  end
end
class C
  int :n
  def initialize(n)
    self.n = n
    while true
      begin
        return :left
      ensure
        self.n = n + 1
      end
    end
  end
end
p A.new(5).n
p A.new(0).n
p B.new(7).n
p C.new(1).n
p A.send(:new, 5).n
`)
	assert.Equals(t, out, "5\n-1\n7\n2\n5\n")
}
