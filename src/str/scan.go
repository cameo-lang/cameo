package str

import (
	"unicode/utf8"
	"unsafe"
)

// ASCIIPrefix returns how many bytes of s from its first are ASCII.
// Blocks of 16 are read with the machine's vector instructions
// (asciiBlocks), the rest a byte at a time.
func ASCIIPrefix(s string) int {
	i := 0
	if len(s) >= 16 {
		i = asciiBlocks(unsafe.StringData(s), len(s))
	}
	for i < len(s) && s[i] < utf8.RuneSelf {
		i++
	}
	return i
}

// ASCIIOnly returns true for a string of 7-bit bytes alone, which reads
// the same in every encoding.
func ASCIIOnly(s string) bool { return ASCIIPrefix(s) == len(s) }

// Count returns the number of characters of s, and false when s is not
// UTF-8. A run of ASCII is passed over in blocks; a string of ASCII
// alone is one run, and its characters are its bytes.
func Count(s string) (int, bool) {
	chars := 0
	for i := 0; i < len(s); {
		if s[i] < utf8.RuneSelf {
			n := ASCIIPrefix(s[i:])
			i += n
			chars += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return 0, false
		}
		i += size
		chars++
	}
	return chars, true
}
