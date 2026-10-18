package protogen

import (
	"hash/fnv"
	"math"
)

// A field's number is the one its declaration gives with number:, from 1
// to MaxDeclaredNumber, or else the one FieldNumber derives from its name,
// from HashedNumberMin to HashedNumberMax. The two ranges do not overlap,
// so a declared number never takes a derived one. A number derived from
// the name stays the same when fields are added, removed or reordered, and
// two programs that each add a field of a different name to the same
// message give them different numbers but by a hash collision.
const (
	MaxDeclaredNumber = 4096
	HashedNumberMin   = 5535
	HashedNumberMax   = HashedNumberMin + hashedNumberSpan
	hashedNumberSpan  = 60000
)

// The numbers protobuf reserves for its implementations.
const (
	firstReservedNumber = 19000
	lastReservedNumber  = 19999
)

// FieldNumber returns the number of a field declared without number::
// HashedNumberMin + 60000 * fnv32a(name) / MaxUint32, moved past
// protobuf's reserved 19000 to 19999 by 1000, or the next number after it
// that taken does not hold, wrapping from HashedNumberMax to
// HashedNumberMin.
func FieldNumber(name string, taken func(int64) bool) int64 {
	h := fnv.New32a()
	h.Write([]byte(name))
	n := HashedNumberMin + int64(uint64(hashedNumberSpan)*uint64(h.Sum32())/math.MaxUint32)
	for {
		if n >= firstReservedNumber && n <= lastReservedNumber {
			n += 1000
		}
		if n > HashedNumberMax {
			n = HashedNumberMin
		}
		if !taken(n) {
			return n
		}
		n++
	}
}
