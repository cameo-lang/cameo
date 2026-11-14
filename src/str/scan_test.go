package str

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/assert"
)

// An encoding has the name a script sees, and ASCII is UTF-8 to one.
func TestAnEncodingHasTheNameAScriptSees(t *testing.T) {
	assert.Equals(t, ASCII.Name(), "UTF-8")
	assert.Equals(t, UTF8.Name(), "UTF-8")
	assert.Equals(t, USASCII.Name(), "US-ASCII")
	assert.Equals(t, Binary.Name(), "ASCII-8BIT")
}

// Binary joined with anything is binary. Text of two encodings shares
// one when a string is empty or ASCII alone, and none when both hold
// other bytes.
func TestWhichEncodingsGoTogether(t *testing.T) {
	joined := func(a Encoding, aEmpty, aASCII bool, b Encoding, bEmpty, bASCII bool) Encoding {
		enc, ok := Joined(a, aEmpty, aASCII, b, bEmpty, bASCII)
		if !ok {
			return 0
		}
		return enc
	}
	assert.Equals(t, joined(UTF8, false, false, Binary, false, true), Binary)
	assert.Equals(t, joined(Binary, true, true, UTF8, false, false), Binary)
	assert.Equals(t, joined(UTF8, false, false, Binary, true, true), Binary)
	assert.Equals(t, joined(UTF8, false, false, UTF8, false, false), UTF8)
	assert.Equals(t, joined(ASCII, false, true, UTF8, false, false), UTF8)
	assert.Equals(t, joined(UTF8, false, false, USASCII, false, true), UTF8)
	assert.Equals(t, joined(USASCII, false, true, UTF8, false, false), UTF8)
	assert.Equals(t, joined(USASCII, false, true, UTF8, false, true), USASCII)
	assert.Equals(t, joined(USASCII, true, true, UTF8, false, true), UTF8)

	_, ok := Shared(UTF8, false, false, Binary, false, false)
	assert.False(t, ok)
	enc, ok := Shared(UTF8, false, false, Binary, false, true)
	assert.True(t, ok)
	assert.Equals(t, enc, UTF8)
	enc, ok = Shared(UTF8, false, true, Binary, false, false)
	assert.True(t, ok)
	assert.Equals(t, enc, Binary)
}

// The scan in blocks finds what a scan byte by byte finds, for a byte
// above 127 at every place of strings of every length around the blocks,
// and from every start within a word.
func TestTheScanInBlocksAgreesWithAScanOfBytes(t *testing.T) {
	plain := strings.Repeat("abcdefgh", 12)
	for start := 0; start < 9; start++ {
		for n := 0; n+start <= len(plain) && n < 70; n++ {
			s := plain[start : start+n]
			assert.Equals(t, ASCIIPrefix(s), n)
			assert.True(t, ASCIIOnly(s))
			for at := 0; at < n; at++ {
				b := []byte(s)
				b[at] = 0x80 + byte(at)
				assert.Equals(t, ASCIIPrefix(string(b)), at)
				assert.False(t, ASCIIOnly(string(b)))
			}
		}
	}
	for _, s := range []string{"", "é", strings.Repeat("aé", 40), strings.Repeat("a", 33) + "€" + strings.Repeat("b", 31) + "😀", strings.Repeat("語", 50)} {
		n, ok := Count(s)
		assert.True(t, ok)
		assert.Equals(t, n, utf8.RuneCountInString(s))
	}
}

// BenchmarkASCIIOnly times the scan of a kilobyte of ASCII.
func BenchmarkASCIIOnly(b *testing.B) {
	s := strings.Repeat("a", 1024)
	b.SetBytes(int64(len(s)))
	for i := 0; i < b.N; i++ {
		if !ASCIIOnly(s) {
			b.Fatal("not ASCII")
		}
	}
}
