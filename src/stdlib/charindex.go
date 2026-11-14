package stdlib

import (
	"sync"
	"unicode/utf8"
	"unsafe"
)

// An index of characters. In a UTF-8 string with characters beyond
// ASCII, character i is not at byte i, and finding it by walking from
// the start makes a loop over the string by index quadratic. The index
// keeps the byte at which every indexStep-th character starts, as far
// as an index has asked for: a lookup starts from the entry before its
// character and walks less than indexStep characters, and a loop over
// the whole string walks each character once.
//
// A value has no room for an index, and is not changed after it is
// made, so the indexes are kept here, for the few strings indexed last,
// by the address and the length of their bytes. An entry holds the
// address, which keeps the bytes from being collected and their address
// from being given to another string meanwhile.

const (
	// indexStep is how many characters lie between two entries.
	indexStep = 64
	// indexFrom is the length in bytes from which a string is indexed;
	// a shorter one is walked.
	indexFrom = 4 * indexStep
	// indexed is how many strings keep an index.
	indexed = 8
)

// charIndex is the index of one string: starts[k] is the byte at which
// character k*indexStep starts.
type charIndex struct {
	data   *byte
	size   int
	starts []int
	used   uint64
}

var charIndexes struct {
	sync.Mutex
	clock uint64
	of    [indexed]charIndex
}

// charOffset returns the byte at which character i of the UTF-8 string
// s starts, or -1 when s has no character i. i is not negative.
func charOffset(s string, i int) int {
	at, rest := 0, i
	if len(s) >= indexFrom {
		at = indexedStart(s, i/indexStep)
		if at < 0 {
			return -1
		}
		rest = i % indexStep
	}
	for ; rest > 0 && at < len(s); rest-- {
		at += charSize(s, at)
	}
	if at >= len(s) {
		return -1
	}
	return at
}

// charSize is the number of bytes of the character that starts at byte
// at of s.
func charSize(s string, at int) int {
	if s[at] < utf8.RuneSelf {
		return 1
	}
	_, size := utf8.DecodeRuneInString(s[at:])
	return size
}

// indexedStart returns the byte at which character k*indexStep of s
// starts, from the index of s, which it makes or extends as far as k,
// or -1 when s has no such character.
func indexedStart(s string, k int) int {
	data := unsafe.StringData(s)
	all := &charIndexes
	all.Lock()
	defer all.Unlock()
	all.clock++
	var ix *charIndex
	oldest := &all.of[0]
	for n := range all.of {
		e := &all.of[n]
		if e.data == data && e.size == len(s) {
			ix = e
			break
		}
		if e.used < oldest.used {
			oldest = e
		}
	}
	if ix == nil {
		ix = oldest
		*ix = charIndex{data: data, size: len(s), starts: append(ix.starts[:0], 0)}
	}
	ix.used = all.clock
	for len(ix.starts) <= k {
		at := ix.starts[len(ix.starts)-1]
		for n := 0; n < indexStep && at < len(s); n++ {
			at += charSize(s, at)
		}
		if at >= len(s) {
			return -1
		}
		ix.starts = append(ix.starts, at)
	}
	return ix.starts[k]
}
