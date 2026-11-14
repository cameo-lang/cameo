package object

import (
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A UTF-8 string knows whether it is ASCII alone, however it was made;
// a string of another kind says nothing, whatever it holds. The mark changes
// neither the string, its length, its kind nor what it is equal to and
// the same as.
func TestAStringKnownToBeASCII(t *testing.T) {
	plain, known := NewString("abc", UTF8), NewString("abc", ASCII)
	assert.True(t, plain.KnownASCII())
	assert.False(t, NewString("abé", UTF8).KnownASCII())
	assert.True(t, NewString("abc", UTF8).KnownASCII())
	assert.True(t, known.KnownASCII())
	assert.True(t, NewString("abc", UTF8).KnownASCII())
	assert.False(t, NewString("abé", UTF8).KnownASCII())
	assert.True(t, NewString("", ASCII).KnownASCII())
	assert.True(t, NewString("", UTF8).KnownASCII())
	assert.False(t, Bytes([]byte("abc")).KnownASCII())
	assert.False(t, NewString("abc", USASCII).KnownASCII())
	assert.False(t, Symbol("abc").KnownASCII())
	assert.False(t, Int(1<<55).KnownASCII())
	assert.False(t, Int(-1).KnownASCII())

	assert.Equals(t, known.Str(), "abc")
	assert.Equals(t, known.Kind(), KindString)
	assert.True(t, Equal(plain, known))
	assert.Equals(t, Key(plain), Key(known))
	s := "shared"
	assert.True(t, Same(NewString(s, UTF8), NewString(s, ASCII)))
	assert.False(t, Same(Int(0), Int(1<<55)))
	assert.False(t, Same(Int(0), Int(1<<40)))
	m := MapOf(known, Int(1))
	v, ok := m.Get(plain)
	assert.True(t, ok)
	assert.Equals(t, v.Int(), int64(1))
}

// A UTF-8 string has its characters counted, by String or by what made
// it, and one made of two has the sum; a string in another encoding has
// no count, nor has a value that is no string.
func TestAStringThatKnowsItsCharacters(t *testing.T) {
	chars := func(v Value) int {
		n, ok := v.Chars()
		if !ok {
			return -1
		}
		return n
	}
	assert.Equals(t, chars(NewString("héllo", UTF8)), 5)
	assert.Equals(t, chars(NewString("héllo", UTF8)), 5)
	assert.Equals(t, chars(StringOfChars("héllo", 5)), 5)
	assert.Equals(t, chars(NewString("abc", ASCII)), 3)
	assert.Equals(t, chars(NewString("", UTF8)), 0)
	assert.Equals(t, chars(Bytes([]byte("abc"))), -1)
	assert.Equals(t, chars(NewString("abc", USASCII)), -1)
	assert.Equals(t, chars(Symbol("abc")), -1)
	assert.Equals(t, chars(Int(-1)), -1)
	assert.Equals(t, chars(Int(5<<32)), -1)

	known := NewString("héllo", UTF8)
	assert.Equals(t, known.Str(), "héllo")
	assert.Equals(t, known.Kind(), KindString)
	assert.False(t, known.KnownASCII())
	assert.True(t, Equal(known, NewString("héllo", UTF8)))
	assert.Equals(t, Key(known), Key(NewString("héllo", UTF8)))
	s := "shéred"
	assert.True(t, Same(NewString(s, UTF8), NewString(s, UTF8)))
	assert.False(t, Same(NewString(s, UTF8), Symbol(s)))

	assert.Equals(t, chars(Joined("héllowörld", known, NewString("wörld", UTF8))), 10)
	assert.Equals(t, chars(Joined("héllo", known, NewString("", UTF8))), 5)
	assert.Equals(t, chars(Joined("héllox", known, NewString("x", UTF8))), 6)
	// Half a character and its other half are one character together.
	assert.Equals(t, chars(Joined("\xc3\xa9", StringOfChars("\xc3", 1), StringOfChars("\xa9", 1))), 1)
	assert.Equals(t, chars(Joined("\xc3a", StringOfChars("\xc3", 1), StringOfChars("a", 1))), 2)
}

// A count too far from the length in bytes to fit beside it is dropped.
func TestACountThatDoesNotFitIsDropped(t *testing.T) {
	long := strings.Repeat("é", 1<<24)
	_, ok := StringOfChars(long, 1<<24).Chars()
	assert.False(t, ok)
	n, ok := StringOfChars(long[:len(long)-4], 1<<24-2).Chars()
	assert.True(t, ok)
	assert.Equals(t, n, 1<<24-2)
	_, ok = StringOfChars("abc", 4).Chars()
	assert.False(t, ok)
}
