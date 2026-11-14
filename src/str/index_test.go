package str

import (
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/cameo-lang/cameo/src/assert"
)

// indexed makes a string of n characters beyond ASCII on the heap,
// indexes it, and returns its key and its index: the string itself is
// not kept.
func indexed(t *testing.T, n int) (indexKey, *charIndex) {
	s := strings.Repeat("é", n)
	assert.Equals(t, Offset(s, n-1), 2*(n-1))
	ix := indexOf(s)
	assert.True(t, ix == indexOf(s), "a second lookup made a second index")
	assert.Equals(t, len(ix.starts), (n-1)/indexStep+1)
	assert.False(t, ix.fixed)
	return indexKey{uintptr(unsafe.Pointer(unsafe.StringData(s))), len(s)}, ix
}

// has returns true while the table has ix under key.
func has(key indexKey, ix *charIndex) bool {
	charIndexes.RLock()
	defer charIndexes.RUnlock()
	return charIndexes.of[key] == ix
}

// The index of a string is made once and kept while the string's bytes
// are there; when they are collected, the index goes too.
func TestAnIndexIsKeptWhileItsBytesAreAndDroppedAfterThem(t *testing.T) {
	key, ix := indexed(t, 1000)
	assert.True(t, has(key, ix))
	for tries := 0; has(key, ix) && tries < 200; tries++ {
		runtime.GC()
		time.Sleep(5 * time.Millisecond)
	}
	assert.False(t, has(key, ix), "the index outlived its bytes")
	assert.True(t, ix.bytes.Value() == nil)
}

// literal is bytes of the program's own data, which the collector does
// not own.
const literal = "ééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé"

// A literal's bytes are not the collector's: its index is kept for good.
func TestALiteralKeepsItsIndex(t *testing.T) {
	assert.True(t, len(literal) >= indexFrom)
	assert.Equals(t, Offset(literal, 100), 200)
	ix := indexOf(literal)
	assert.True(t, ix.fixed)
	runtime.GC()
	assert.True(t, ix == indexOf(literal))
	assert.Equals(t, Offset(literal, 129), 258)
	assert.Equals(t, Offset(literal, 200), -1)
}

// An index is that of the bytes it was made for alone: one left under
// an address whose bytes are gone is not used for the bytes that came
// to the address after them.
func TestAnIndexOfBytesThatAreGoneIsNotUsed(t *testing.T) {
	s := strings.Repeat("é", 500)
	stale := &charIndex{starts: []int{0, 7}} // of other bytes: its weak pointer is unset
	key := indexKey{uintptr(unsafe.Pointer(unsafe.StringData(s))), len(s)}
	charIndexes.Lock()
	if charIndexes.of == nil {
		charIndexes.of = map[indexKey]*charIndex{}
	}
	charIndexes.of[key] = stale
	charIndexes.Unlock()
	assert.Equals(t, Offset(s, 70), 140)
	assert.True(t, indexOf(s) != stale)
	runtime.KeepAlive(s)
}
