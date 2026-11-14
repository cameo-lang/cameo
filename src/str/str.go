// Package str knows what the bytes of a string are: the encodings a
// string is made in, the scan that tells ASCII from other text and
// counts characters, the index that finds a character in UTF-8, and
// which encodings go together when strings are joined. It knows no
// value of the language: package object makes its strings with what is
// here.
package str

// Encoding says what the bytes of a string are. Every string is made
// with one, so no string is of unknown kind.
type Encoding uint8

const (
	// ASCII is 7-bit bytes alone, which the maker knows: digits, hex,
	// a name of the language. A script sees a UTF-8 string, and its
	// characters are its bytes.
	ASCII Encoding = iota + 1
	// UTF8 is text that may hold characters beyond ASCII. The scan
	// that checks it counts its characters, so the string knows
	// whether it is ASCII alone.
	UTF8
	// USASCII is a string a script labelled US-ASCII.
	USASCII
	// Binary is bytes without characters.
	Binary
)

// UnknownLength is the number of characters of a string that nobody
// counted.
const UnknownLength = -1

// Name is the name of the encoding as a script sees it. ASCII is UTF-8
// to a script: that a string holds nothing else is what the string
// knows, not what it is labelled.
func (e Encoding) Name() string {
	switch e {
	case USASCII:
		return "US-ASCII"
	case Binary:
		return "ASCII-8BIT"
	}
	return "UTF-8"
}

// text is e as far as joining goes: a string of ASCII alone is UTF-8.
func (e Encoding) text() Encoding {
	if e == ASCII {
		return UTF8
	}
	return e
}

// Shared is the encoding two strings read as characters share, as
// Ruby's rb_enc_compatible decides it: either's when they share it, the
// other's when one is empty or ASCII alone, and none when both hold
// other bytes in different encodings. Each string is given as its
// encoding, whether it is empty, and whether it holds ASCII alone.
func Shared(a Encoding, aEmpty, aASCII bool, b Encoding, bEmpty, bASCII bool) (Encoding, bool) {
	a, b = a.text(), b.text()
	switch {
	case a == b, bEmpty:
		return a, true
	case aEmpty:
		return b, true
	case bASCII:
		return a, true
	case aASCII:
		return b, true
	}
	return 0, false
}

// Joined is the encoding of a string made by joining two. Binary joined
// with anything is binary, whatever either holds: bytes do not become
// characters by standing beside some. Ruby makes text of the two when
// the binary one is ASCII alone, and raises when both hold other bytes.
// Text of two encodings goes together as Shared has it.
func Joined(a Encoding, aEmpty, aASCII bool, b Encoding, bEmpty, bASCII bool) (Encoding, bool) {
	if a == Binary || b == Binary {
		return Binary, true
	}
	return Shared(a, aEmpty, aASCII, b, bEmpty, bASCII)
}
