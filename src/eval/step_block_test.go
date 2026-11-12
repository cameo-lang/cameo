package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A builtin that calls its block step by step runs every call of the
// block on one activation when a call leaves nothing behind. A call
// then starts as the first did: a variable of the block is unset again,
// a begin it left by next is not open, and what broke out or raised
// ends the builtin as before.
func TestABlocksCallsOnOneActivationStartAfresh(t *testing.T) {
	out, err := run(t, `
[1, 2, 3].each { |x| y = 10 if x == 1; p y }
3.times do |i|
  begin
    next if i == 1
    p i
  ensure
    p :left
  end
end
p (1..5).each { |i| break i if i == 3 }
r = begin
  [1, 2].each { |x| raise "at #{x}" if x == 2 }
rescue => e
  e.message
end
p r
[[1, 2], [3]].each { |a, b| p [a, b] }
[1, 2].each { p :none }
[1, 2].each_with_index { |x| p x }
p [1, 2, 3].map { |x| [4, 5].map { |z| z * x } }
def twice(l) = l.each { |x| return x * 2 if x > 1 }
p twice([1, 2, 3])
[1, 2].each { |x| x = x + 1; p x }
`)
	assert.NoError(t, err)
	assert.Equals(t, out, `10
nil
nil
0
:left
:left
2
:left
3
"at 2"
[1, 2]
[3, nil]
:none
:none
1
2
[[4, 5], [8, 10], [12, 15]]
4
2
3
`)
}
