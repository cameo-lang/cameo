package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/object"
)

// A machine that is kept for the next call holds nothing of the call it
// ran: no value, no cursor, no frame.
func TestAReleasedMachineHoldsNothing(t *testing.T) {
	in := New(nil, nil)
	m := in.machine()
	m.kind, m.fr, m.env = actMethod, &Frame{}, NewEnv(nil)
	m.push(object.String("kept alive"))
	m.push(object.Int(1))
	m.pop()
	m.cursors = append(m.cursors, cursor{pending: ErrStopped})
	m.cursors = m.cursors[:0]
	in.release(m)

	again := in.machine()
	assert.True(t, again == m)
	assert.True(t, again.in == in)
	assert.True(t, again.fr == nil && again.env == nil && again.kind == actList)
	assert.Equals(t, len(again.stack), 0)
	assert.Equals(t, len(again.cursors), 0)
	for _, v := range again.stack[:cap(again.stack)] {
		assert.True(t, v.IsZero())
	}
	for _, c := range again.cursors[:cap(again.cursors)] {
		assert.True(t, c.pending == nil)
	}
}

// A machine whose stack grew large is not kept, and no more machines are
// kept than maxFree.
func TestMachinesKeptAreBounded(t *testing.T) {
	in := New(nil, nil)
	big := in.machine()
	big.stack = make([]object.Value, 0, maxKeptStack+1)
	in.release(big)
	assert.Equals(t, len(in.free), 0)

	for i := 0; i < maxFree+10; i++ {
		in.release(&machine{in: in})
	}
	assert.Equals(t, len(in.free), maxFree)
}

// Calls that run on reused machines return what they returned on fresh
// ones: recursion deeper than the machines kept, blocks, lambdas, a
// method left by an exception and one left by a break.
func TestCallsOnReusedMachines(t *testing.T) {
	out, err := run(t, `def fib(n) = n < 3 ? 1 : fib(n - 1) + fib(n - 2)
def down(n) = n == 0 ? 0 : 1 + down(n - 1)
def failing(n)
  raise "at #{n}" if n == 0
  failing(n - 1)
end
def each_twice
  yield 1
  yield 2
end
def first_over(limit)
  [1, 5, 9].each { |x| return x if x > limit }
  nil
end
puts fib(20)
puts down(1000)
puts down(1000)
begin
  failing(50)
rescue => e
  puts e.message
end
puts fib(15)
each_twice { |x| puts x * 10 }
add = ->(a, b) { a + b }
puts add.(1, 2), add.(3, 4)
puts first_over(4)
puts [3, 1, 2].sort { |a, b| b <=> a }.inspect
puts "a" + "b", 2.5 * 2, 7 / 2, 2**70
`)
	assert.NoError(t, err)
	assert.Equals(t, out, "6765\n1000\n1000\nat 0\n610\n10\n20\n3\n7\n5\n[3, 2, 1]\nab\n5.0\n3\n1180591620717411303424\n")
}
