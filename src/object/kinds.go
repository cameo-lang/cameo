package object

import (
	"math"
	"sync/atomic"
)

// Kinds is a set of Kinds: the kinds the elements of a collection hold.
// One more bit marks a NaN among its floats, which compares with nothing.
type Kinds uint32

// KindsNaN is set when a Float among the elements is NaN.
const KindsNaN Kinds = 1 << 31

// kindsKnown marks a List's cached Kinds as computed; it is never part of
// a set that Kinds returns.
const kindsKnown Kinds = 1 << 30

// KindsOf computes the kinds of elems.
func KindsOf(elems []Value) Kinds {
	var k Kinds
	for _, el := range elems {
		k |= 1 << el.kind
		if el.kind == KindFloat && math.IsNaN(el.Float()) {
			k |= KindsNaN
		}
	}
	return k
}

// Only reports a non-empty set of nothing but the given kinds.
func (k Kinds) Only(kinds ...Kind) bool {
	var mask Kinds
	for _, kind := range kinds {
		mask |= 1 << kind
	}
	return k != 0 && k&^mask == 0
}

// Kinds returns the kinds of the list's elements. It is computed on the
// first call and kept, which is sound because Elems never changes.
func (l *List) Kinds() Kinds {
	if k := Kinds(atomic.LoadUint32(&l.kinds)); k&kindsKnown != 0 {
		return k &^ kindsKnown
	}
	k := KindsOf(l.Elems)
	atomic.StoreUint32(&l.kinds, uint32(k|kindsKnown))
	return k
}
