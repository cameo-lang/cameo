package object

import (
	"github.com/cameo-lang/cameo/src/internal/trusted"
	"github.com/cameo-lang/cameo/src/str"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// wrongEncoding returns what NewString panics with for s made as enc,
// nil when it makes the string.
func wrongEncoding(s string, enc str.Encoding) (wrong *WrongEncoding) {
	defer func() {
		if r := recover(); r != nil {
			wrong = r.(*WrongEncoding)
		}
	}()
	NewString(s, enc)
	return nil
}

// NewString reads the bytes and refuses those that are not what the
// encoding says: a byte above 127 as ASCII or US-ASCII, and bytes that
// are not UTF-8 as UTF-8. str.Binary takes any.
func TestNewStringRefusesBytesThatAreNotTheEncoding(t *testing.T) {
	for _, enc := range []str.Encoding{str.ASCII, str.USASCII} {
		assert.Nil(t, wrongEncoding("", enc))
		assert.Nil(t, wrongEncoding("plain", enc))
		assert.NotNil(t, wrongEncoding("h\xc3\xa9llo", enc))
		assert.NotNil(t, wrongEncoding(strings.Repeat("a", 100)+"\x80", enc))
		assert.NotNil(t, wrongEncoding("\xff"+strings.Repeat("a", 100), enc))
	}
	assert.Nil(t, wrongEncoding("h\xc3\xa9llo", str.UTF8))
	assert.Nil(t, wrongEncoding(strings.Repeat("a", 100)+"\xe2\x82\xac", str.UTF8))
	assert.NotNil(t, wrongEncoding("\xff", str.UTF8))
	assert.NotNil(t, wrongEncoding(strings.Repeat("a", 100)+"\xc3", str.UTF8))
	assert.NotNil(t, wrongEncoding("\xed\xa0\x80", str.UTF8)) // a surrogate
	assert.Nil(t, wrongEncoding("\xff\x00", str.Binary))

	assert.Equals(t, wrongEncoding("h\xc3\xa9", str.ASCII).Error(), `invalid byte sequence in ASCII: "hé" is not text; keep such bytes in a binary string`)
	assert.Equals(t, wrongEncoding("a\xff", str.USASCII).Error(), `invalid byte sequence in US-ASCII: "a\xff" is not text; keep such bytes in a binary string`)
	assert.Equals(t, wrongEncoding("a\xff", str.UTF8).Error(), `invalid byte sequence in UTF-8: "a\xff" is not text; keep such bytes in a binary string`)
}

// A string made as UTF-8 knows whether it is ASCII alone, and one made
// as ASCII is: both are UTF-8 strings to a script.
func TestAUTF8StringKnowsWhetherItIsASCII(t *testing.T) {
	plain, text := NewString("plain", str.UTF8), NewString("h\xc3\xa9llo", str.UTF8)
	assert.True(t, plain.KnownASCII())
	assert.False(t, text.KnownASCII())
	assert.True(t, NewString("plain", str.ASCII).KnownASCII())
	assert.Equals(t, plain.Kind(), KindString)
	assert.Equals(t, NewString("plain", str.ASCII).Kind(), KindString)
	assert.Equals(t, text.Chars(), 5)

	v, ok := TryUTF8("h\xc3\xa9llo")
	assert.True(t, ok)
	assert.Equals(t, v.Kind(), KindString)
	_, ok = TryUTF8("\xc3")
	assert.False(t, ok)
}

// TrustedString makes a string of the encoding it is told without
// reading the bytes: with the count it is given, with none when the
// caller has none, and with the bytes as the characters of ASCII.
func TestTrustedStringTakesTheCallersWord(t *testing.T) {
	key := trusted.Key{}
	counted := TrustedString(key, "h\xc3\xa9llo", str.UTF8, 5)
	assert.Equals(t, counted.Kind(), KindString)
	assert.Equals(t, counted.Chars(), 5)
	assert.False(t, counted.KnownASCII())

	uncounted := TrustedString(key, "h\xc3\xa9llo", str.UTF8, str.UnknownLength)
	assert.Equals(t, uncounted.Kind(), KindString)
	assert.Equals(t, uncounted.Chars(), str.UnknownLength)
	assert.False(t, uncounted.KnownASCII())
	assert.True(t, Equal(counted, uncounted))

	ascii := TrustedString(key, "plain", str.ASCII, str.UnknownLength)
	assert.Equals(t, ascii.Chars(), 5)
	assert.True(t, ascii.KnownASCII())

	assert.Equals(t, TrustedString(key, "ab", str.USASCII, 2).Kind(), KindASCII)
	assert.Equals(t, TrustedString(key, "\xff", str.Binary, str.UnknownLength).Kind(), KindBytes)
	assert.Equals(t, TrustedString(key, "\xff", str.Binary, str.UnknownLength).Chars(), str.UnknownLength)

	// It does not look: a caller that is wrong gets what it said.
	assert.Equals(t, TrustedString(key, "\xff", str.UTF8, 1).Kind(), KindString)
}
