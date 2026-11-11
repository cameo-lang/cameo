package object

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The key a lookup builds without allocating is the key Key returns, for
// every value it builds one for, and a lookup finds what a key set.
func TestQuickKeyIsKey(t *testing.T) {
	long := string(make([]byte, quickKeySize))
	values := []Value{Int(0), Int(-1), Int(1 << 62), Int(-1 << 63), Symbol(""), Symbol("name"), Symbol("héllo"),
		String(""), String("a"), String("héllo"), String("12:34"), String(long[:quickKeySize-24])}
	for _, v := range values {
		var buf [quickKeySize]byte
		key, ok := quickKey(buf[:0], v)
		assert.True(t, ok)
		assert.Equals(t, string(key), Key(v))
	}
	for _, v := range []Value{Float(1), Nil, True, String(long), StringOf(KindASCII, "a"), Bytes([]byte("a")), NewList(Int(1))} {
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
