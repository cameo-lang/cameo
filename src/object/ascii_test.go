package object

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A string made as ASCII, or a literal that is, is known to be; any
// other string says nothing, whatever it holds. The mark changes
// neither the string, its length, its kind nor what it is equal to and
// the same as.
func TestAStringKnownToBeASCII(t *testing.T) {
	plain, known := String("abc"), ASCIIString("abc")
	assert.False(t, plain.KnownASCII())
	assert.True(t, known.KnownASCII())
	assert.True(t, Literal("abc").KnownASCII())
	assert.False(t, Literal("abé").KnownASCII())
	assert.True(t, ASCIIString("").KnownASCII())
	assert.True(t, String("").KnownASCII())
	assert.False(t, Bytes([]byte("abc")).KnownASCII())
	assert.False(t, StringOf(KindASCII, "abc").KnownASCII())
	assert.False(t, Symbol("abc").KnownASCII())
	assert.False(t, Int(1<<55).KnownASCII())
	assert.False(t, Int(-1).KnownASCII())

	assert.Equals(t, known.Str(), "abc")
	assert.Equals(t, known.Kind(), KindString)
	assert.True(t, Equal(plain, known))
	assert.Equals(t, Key(plain), Key(known))
	s := "shared"
	assert.True(t, Same(String(s), ASCIIString(s)))
	assert.False(t, Same(Int(0), Int(1<<55)))
	m := MapOf(known, Int(1))
	v, ok := m.Get(plain)
	assert.True(t, ok)
	assert.Equals(t, v.Int(), int64(1))
}
