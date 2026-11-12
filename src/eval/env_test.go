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

func syms(names ...string) []object.Sym {
	out := make([]object.Sym, len(names))
	for i, name := range names {
		out[i] = object.Intern(name)
	}
	return out
}

// A scope without the layout of a proc gives each name it binds a slot
// of its own layout, beyond the ones it holds inline and beyond the
// number a layout is looked through for.
func TestScopeGrowsItsOwnLayout(t *testing.T) {
	e := NewEnv(nil)
	_, ok := e.Get("a")
	assert.True(t, !ok)
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	for i, name := range names {
		e.Define(name, object.Int(int64(i)))
	}
	e.Set("c", object.Int(30))
	for i, name := range names {
		v, ok := e.Get(name)
		assert.True(t, ok)
		want := int64(i)
		if name == "c" {
			want = 30
		}
		assert.Equals(t, v.Int(), want)
	}
	assert.True(t, e.own && e.layout.index != nil)
	var seen []string
	e.each(func(name string, _ object.Value) { seen = append(seen, name) })
	assert.Equals(t, seen, names)
}

// A scope with the layout of a proc holds the names of the layout in
// slots, where an empty slot is an unbound name, and any other name in
// a map.
func TestScopeOfAProcLayout(t *testing.T) {
	l := newLayout(syms("a", "b", "a"))
	assert.Equals(t, len(l.names), 2)
	e := newEnvOf(nil, l)
	_, ok := e.Get("b")
	assert.True(t, !ok)
	e.Define("b", object.Int(2))
	e.Define("other", object.Int(3))
	assert.Equals(t, e.slots[1].Int(), int64(2))
	assert.True(t, e.layout == l && !e.own && len(l.names) == 2)
	v, ok := e.Get("other")
	assert.True(t, ok && v.Int() == 3)
	var seen []string
	e.each(func(name string, _ object.Value) { seen = append(seen, name) })
	assert.Equals(t, seen, []string{"b", "other"})
	// A slot reserved at the top level holds nothing until it is bound.
	top := NewEnv(nil)
	assert.Equals(t, top.reserve(object.Intern("x")), 0)
	_, ok = top.Get("x")
	assert.True(t, !ok)
	assert.Equals(t, e.reserve(object.Intern("nowhere")), -1)
}

// capture copies the variables a block reads, the innermost binding of
// each, from the scopes up to the first captured one, which becomes the
// parent.
func TestCaptureCopiesWhatABlockReads(t *testing.T) {
	outer := NewEnv(nil)
	outer.Define("a", object.Int(1))
	outer.Define("b", object.Int(2))
	outer.Define("unread", object.Int(9))
	l := newLayout(syms("x", "a", "b"))
	first := outer.capture(l, []int32{1, 2})
	assert.True(t, first.captured && first.parent == nil && first.scope == outer && first.layout == l)
	assert.True(t, first.slots[0].IsZero() && first.slots[1].Int() == 1 && first.slots[2].Int() == 2)
	_, ok := first.Get("unread")
	assert.True(t, !ok)

	// A block written in a call of the first: its call rebinds b, and
	// the inner block reads a from the first's captured scope.
	call := newEnvOf(first, l)
	call.SetSym(object.Intern("b"), object.Int(20))
	inner := newLayout(syms("b", "a"))
	second := call.capture(inner, []int32{0, 1})
	assert.True(t, second.parent == first && second.scope == outer)
	assert.True(t, second.slots[0].Int() == 20 && second.slots[1].IsZero())
	v, ok := second.Get("a")
	assert.True(t, ok && v.Int() == 1)
	// The first captured scope is as it was.
	assert.Equals(t, first.slots[2].Int(), int64(2))
}
