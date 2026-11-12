package object

import (
	"sync/atomic"
	"unsafe"
)

// A grown list is one Append made. Its elements are the first of a store
// that has room for more, and every list over the store is such a
// prefix, so that the next Append writes behind its list's last element
// instead of copying them all.
type grown struct {
	List
	store *store
}

// store is the elements of grown lists and how many of them a list has:
// the slots behind used are free. An Append takes the slots behind its
// list by moving used on from the list's length, which works for the
// longest list alone. Appending to a shorter one copies, as its slots
// are another list's.
type store struct {
	slots []Value
	used  atomic.Int64
}

// Append returns the list of l's elements and then xs. The list is new
// and l is as it was, as for every list, but making it costs the
// elements of xs alone when l is the longest list of a store with room,
// which the list a script appends to in a loop is: n appends cost in
// the order of n, not of n squared.
//
// A grown list keeps its store reachable, which is up to half as large
// again as the longest list made in it.
func Append(l *List, xs ...Value) Value {
	n := len(l.Elems)
	need := n + len(xs)
	if l.grown {
		st := (*grown)(unsafe.Pointer(l)).store
		if need <= len(st.slots) && st.used.CompareAndSwap(int64(n), int64(need)) {
			copy(st.slots[n:need], xs)
			return ListValue(&(&grown{List{Elems: st.slots[:need:need], grown: true}, st}).List)
		}
	}
	st := &store{slots: make([]Value, need+max(need/2, 4))}
	copy(st.slots, l.Elems)
	copy(st.slots[n:], xs)
	st.used.Store(int64(need))
	return ListValue(&(&grown{List{Elems: st.slots[:need:need], grown: true}, st}).List)
}
