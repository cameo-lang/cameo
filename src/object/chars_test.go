package object

import (
	"strings"
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
	assert.False(t, Same(Int(0), Int(1<<40)))
	m := MapOf(known, Int(1))
	v, ok := m.Get(plain)
	assert.True(t, ok)
	assert.Equals(t, v.Int(), int64(1))
}

// A string made with its characters counted has them, and one made of
// two such has the sum; a string nobody counted says so, as does one in
// another encoding, and a value that is no string.
func TestAStringThatKnowsItsCharacters(t *testing.T) {
	chars := func(v Value) int {
		n, ok := v.Chars()
		if !ok {
			return -1
		}
		return n
	}
	assert.Equals(t, chars(String("héllo")), -1)
	assert.Equals(t, chars(Literal("héllo")), 5)
	assert.Equals(t, chars(StringOfChars("héllo", 5)), 5)
	assert.Equals(t, chars(ASCIIString("abc")), 3)
	assert.Equals(t, chars(String("")), 0)
	assert.Equals(t, chars(Bytes([]byte("abc"))), -1)
	assert.Equals(t, chars(StringOf(KindASCII, "abc")), -1)
	assert.Equals(t, chars(Symbol("abc")), -1)
	assert.Equals(t, chars(Int(-1)), -1)
	assert.Equals(t, chars(Int(5<<34)), -1)

	known := Literal("héllo")
	assert.Equals(t, known.Str(), "héllo")
	assert.Equals(t, known.Kind(), KindString)
	assert.False(t, known.KnownASCII())
	assert.True(t, Equal(known, String("héllo")))
	assert.Equals(t, Key(known), Key(String("héllo")))
	s := "shéred"
	assert.True(t, Same(String(s), Literal(s)))
	assert.False(t, Same(String(s), Symbol(s)))

	assert.Equals(t, chars(Joined("héllowörld", known, Literal("wörld"))), 10)
	assert.Equals(t, chars(Joined("héllo", known, String(""))), 5)
	assert.Equals(t, chars(Joined("héllox", known, String("x"))), -1)
	// Half a character and its other half are one character together.
	assert.Equals(t, chars(Joined("\xc3\xa9", Literal("\xc3"), Literal("\xa9"))), -1)
	assert.Equals(t, chars(Joined("\xc3a", Literal("\xc3"), Literal("a"))), 2)
}

// A count too far from the length in bytes to fit beside it is dropped.
func TestACountThatDoesNotFitIsDropped(t *testing.T) {
	long := strings.Repeat("é", 1<<22)
	_, ok := StringOfChars(long, 1<<22).Chars()
	assert.False(t, ok)
	n, ok := StringOfChars(long[:len(long)-4], 1<<22-2).Chars()
	assert.True(t, ok)
	assert.Equals(t, n, 1<<22-2)
	_, ok = StringOfChars("abc", 4).Chars()
	assert.False(t, ok)
}
