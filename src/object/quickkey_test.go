package object

import (
	"math/big"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The key a lookup builds without allocating is the key Key returns, for
// every value it builds one for, and a lookup finds what a key set.
func TestQuickKeyIsKey(t *testing.T) {
	long := string(make([]byte, quickKeySize))
	values := []Value{Symbol(""), Symbol("name"), Symbol("héllo"),
		String(""), String("a"), String("héllo"), String("12:34"), String(long[:quickKeySize-24])}
	for _, v := range values {
		var buf [quickKeySize]byte
		key, ok := quickKey(buf[:0], v)
		assert.True(t, ok)
		assert.Equals(t, string(key), Key(v))
	}
	for _, v := range []Value{Int(0), Int(-1), Float(1), Nil, True, String(long), StringOf(KindASCII, "a"), Bytes([]byte("a")), NewList(Int(1))} {
		var buf [quickKeySize]byte
		_, ok := quickKey(buf[:0], v)
		assert.False(t, ok)
	}

	m := MapOf(Int(1), Int(10), Symbol("a"), Int(20), String("a"), Int(30), Float(1), Int(40), String(long), Int(50))
	for i, k := range []Value{Int(1), Symbol("a"), String("a"), Float(1), String(long)} {
		v, ok := m.Get(k)
		assert.True(t, ok)
		assert.Equals(t, v.Int(), int64(10*(i+1)))
	}
	assert.False(t, m.Has(Int(2)))
	assert.False(t, m.Has(Symbol("b")))
	assert.True(t, m.Without(Int(1)).Has(Symbol("a")))
	assert.False(t, m.Without(Int(1)).Has(Int(1)))
	assert.Equals(t, int(testing.AllocsPerRun(100, func() { m.Get(Int(1)) })), 0)
	assert.Equals(t, int(testing.AllocsPerRun(100, func() { m.Get(Symbol("a")) })), 0)
}

// An integer key is found by its number and has no canonical form made
// for it: setting one allocates no key, an integer and a float of the
// same magnitude are two keys, as are an integer and a big integer, and
// a map and a set keep their order and their deletions.
func TestIntegerKeysAreFoundByNumber(t *testing.T) {
	assert.Equals(t, canon(Int(7)), "")
	big := BigInt(new(big.Int).Lsh(big.NewInt(1), 70))
	m := NewMap().Put(Int(3), String("three")).Put(Float(3), String("float")).Put(Int(-1<<63), String("min")).Put(big, String("big")).Put(Int(1), String("one"))
	for k, want := range map[int64]string{3: "three", -1 << 63: "min", 1: "one"} {
		v, ok := m.Get(Int(k))
		assert.True(t, ok)
		assert.Equals(t, v.Str(), want)
	}
	v, _ := m.Get(Float(3))
	assert.Equals(t, v.Str(), "float")
	v, _ = m.Get(big)
	assert.Equals(t, v.Str(), "big")
	assert.Equals(t, m.Len(), 5)
	assert.False(t, m.Has(Int(2)))

	// A key set again keeps its place; one deleted and added goes last.
	m = m.Put(Int(3), String("again")).Without(Int(1)).Put(Int(1), String("back"))
	keys := m.Keys()
	assert.Equals(t, Key(keys[0]), Key(Int(3)))
	assert.Equals(t, Key(keys[len(keys)-1]), Key(Int(1)))
	// The version before is as it was.
	old := NewMap().Put(Int(1), Int(10))
	next := old.Put(Int(1), Int(20))
	a, _ := old.Get(Int(1))
	b, _ := next.Get(Int(1))
	assert.Equals(t, a.Int()+b.Int(), int64(30))

	s := NewSet(Int(1), Int(2), Int(1))
	assert.Equals(t, s.Len(), 2)
	assert.True(t, s.Has(Int(2)) && !s.Has(Int(3)) && !s.Without(Int(2)).Has(Int(2)))

	sized := NewMapSized(8)
	assert.Equals(t, int(testing.AllocsPerRun(100, func() { sized.Add(Int(5), Int(1)) })), 0)
}
