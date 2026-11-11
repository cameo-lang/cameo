package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/object"
)

// A block written in a method's scope captures a copy of its variables.
// A block written inside a call of that block copies the call's own
// variables and has the captured scope as its parent.
func TestASnapshotSharesTheCapturedScopeItSitsIn(t *testing.T) {
	method := NewEnv(nil)
	method.Define("a", object.Int(1))
	outer := method.snapshot()
	assert.True(t, outer.captured && outer.parent == nil && outer.scope == method)

	call := NewEnv(outer)
	call.Define("x", object.Int(2))
	inner := call.snapshot()
	assert.True(t, inner.captured && inner.parent == outer && inner.scope == method)
	_, own := inner.lookup("a")
	assert.False(t, own)
	a, _ := inner.Get("a")
	x, _ := inner.Get("x")
	assert.Equals(t, a, object.Int(1))
	assert.Equals(t, x, object.Int(2))

	// What the scopes bind afterwards does not reach the snapshot.
	method.Set("a", object.Int(10))
	call.Set("x", object.Int(20))
	call.Set("a", object.Int(30))
	a, _ = inner.Get("a")
	x, _ = inner.Get("x")
	assert.Equals(t, a, object.Int(1))
	assert.Equals(t, x, object.Int(2))
}

// A scope between the one a block is written in and the captured scope
// is copied too: the body of a for loop runs in a scope of its own inside
// the scope around it.
func TestASnapshotCopiesEveryScopeUpToTheCapturedOne(t *testing.T) {
	captured := NewEnv(nil).snapshot()
	call := NewEnv(captured)
	call.Define("x", object.Int(1))
	call.Define("y", object.Int(2))
	loop := NewEnv(call)
	loop.Define("x", object.Int(3))
	s := loop.snapshot()
	assert.True(t, s.parent == captured)
	x, _ := s.lookup("x")
	y, _ := s.lookup("y")
	assert.Equals(t, x, object.Int(3))
	assert.Equals(t, y, object.Int(2))
}

// Blocks nested in calls of blocks read what each level captured, and a
// lambda that names itself reads itself.
func TestNestedBlocksReadWhatEachLevelCaptured(t *testing.T) {
	expectOutput(t, `add = ->(a) { ->(b) { ->(c) { a + b + c } } }
puts add.(1).(2).(3)
k = 10
scale = ->(x) { [1, 2].map { |y| [3].map { |z| x * y * z * k } } }
k = 1000
puts scale.(2).inspect
fact = ->(n) { n < 2 ? 1 : n * fact.(n - 1) }
puts fact.(5)
shadow = ->(k) { -> { k } }
puts shadow.(7).()
`, "6\n[[60], [120]]\n120\n7\n")
}
