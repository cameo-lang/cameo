package str

import (
	"runtime"
	"sync"
	"unicode/utf8"
	"unsafe"
	"weak"
)

// An index of characters, for Offset. In a UTF-8 string with characters beyond
// ASCII, character i is not at byte i, and finding it by walking from
// the start makes a loop over the string by index quadratic. The index
// keeps the byte at which every indexStep-th character starts, as far
// as an index has asked for: a lookup starts from the entry before its
// character and walks less than indexStep characters, and a loop over
// the whole string walks each character once.
//
// The index is made when a string is first indexed, and kept for as
// long as the string's bytes are: a value is not changed after it is
// made, so what was found out about its bytes stays true. A value has
// no room for an index, so the indexes are kept here, by the address
// and the length of the bytes. An index does not keep the bytes alive:
// it holds a weak pointer to them, which says when they are gone, and
// the collector removes the index after them.

const (
	// indexStep is how many characters lie between two entries.
	indexStep = 64
	// indexFrom is the length in bytes from which a string is indexed;
	// a shorter one is walked.
	indexFrom = 4 * indexStep
)

// indexKey names the bytes of a string: where they are and how many.
type indexKey struct {
	addr uintptr
	size int
}

// charIndex is the index of one string: starts[k] is the byte at which
// character k*indexStep starts.
type charIndex struct {
	sync.Mutex
	// bytes is the string's bytes while they are there. The bytes of
	// the program's own data, a literal's, are there always: fixed
	// says so, and bytes is then unset.
	bytes  weak.Pointer[byte]
	fixed  bool
	starts []int
}

var charIndexes struct {
	sync.RWMutex
	of map[indexKey]*charIndex
}

// Offset returns the byte at which character i of the UTF-8 string
// s starts, or -1 when s has no character i. i is not negative.
func Offset(s string, i int) int {
	at, rest := 0, i
	if len(s) >= indexFrom {
		at = indexOf(s).start(s, i/indexStep)
		if at < 0 {
			return -1
		}
		rest = i % indexStep
	}
	for ; rest > 0 && at < len(s); rest-- {
		at += CharSize(s, at)
	}
	if at >= len(s) {
		return -1
	}
	return at
}

// CharSize is the number of bytes of the character that starts at byte
// at of s.
func CharSize(s string, at int) int {
	if s[at] < utf8.RuneSelf {
		return 1
	}
	_, size := utf8.DecodeRuneInString(s[at:])
	return size
}

// start returns the byte at which character k*indexStep of s starts,
// extending the index as far as k, or -1 when s has no such character.
func (ix *charIndex) start(s string, k int) int {
	ix.Lock()
	defer ix.Unlock()
	for len(ix.starts) <= k {
		at := ix.starts[len(ix.starts)-1]
		for n := 0; n < indexStep && at < len(s); n++ {
			at += CharSize(s, at)
		}
		if at >= len(s) {
			return -1
		}
		ix.starts = append(ix.starts, at)
	}
	return ix.starts[k]
}

// indexOf returns the index of s, which it makes when s has none.
func indexOf(s string) *charIndex {
	data := unsafe.StringData(s)
	key := indexKey{uintptr(unsafe.Pointer(data)), len(s)}
	all := &charIndexes
	all.RLock()
	ix := all.of[key]
	all.RUnlock()
	if ix != nil && ix.of(data) {
		return ix
	}
	all.Lock()
	defer all.Unlock()
	// Another worker may have made it meanwhile.
	if ix = all.of[key]; ix != nil && ix.of(data) {
		return ix
	}
	ix = &charIndex{starts: []int{0}}
	if all.of == nil {
		all.of = map[indexKey]*charIndex{}
	}
	all.of[key] = ix
	// The collector tells when the bytes are gone, for bytes it
	// allocated; it refuses the program's own data, which stays.
	if !cleanedUp(data, key, ix) {
		ix.fixed = true
		return ix
	}
	ix.bytes = weak.Make(data)
	return ix
}

// of returns true while ix is the index of the bytes at data: the bytes
// it was made for are still there. Bytes that are gone may have left
// their address to another string before the index was removed.
func (ix *charIndex) of(data *byte) bool {
	return ix.fixed || ix.bytes.Value() == data
}

// dropped is what the collector is given to remove an index with.
type dropped struct {
	key indexKey
	ix  *charIndex
}

// cleanedUp asks the collector to remove ix once the bytes at data are
// gone, and returns false when the bytes are none of the collector's.
func cleanedUp(data *byte, key indexKey, ix *charIndex) (ok bool) {
	defer func() { ok = recover() == nil }()
	runtime.AddCleanup(data, dropIndex, dropped{key, ix})
	return true
}

// dropIndex removes an index whose bytes are gone, unless the address
// has an index of other bytes by now.
func dropIndex(d dropped) {
	all := &charIndexes
	all.Lock()
	if all.of[d.key] == d.ix {
		delete(all.of, d.key)
	}
	all.Unlock()
}
