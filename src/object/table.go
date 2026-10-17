package object

import (
	"sync"
	"sync/atomic"
)

// A table is the storage maps and sets share: an append-only log of
// entries and an index from each key to its latest entry. A map or a set
// is a version of a table, its first n entries. The version at the table's
// tip grows the table in place, so building a map key by key, as
// `h = h.put(k, v)` in a loop does, costs O(1) per key instead of a copy;
// any other version copies its live entries into a new table first. An
// entry below a version's length never changes, so a version is immutable
// however the table grows. The index and the log grow under the table's
// lock, since a constant's map can be read by several processes at once.
type table struct {
	mu      sync.RWMutex
	entries []entry
	index   map[string]int32
	// live counts the keys present at the tip, to know when the log is
	// mostly superseded entries and should be compacted.
	live int
}

// An entry sets a key to a value, or deletes it. prev is the key's
// previous entry and start the entry its current life began at: a key's
// place in the order is where it was last added after being absent.
type entry struct {
	k     string
	key   Value
	val   Value
	prev  int32
	start int32
	gone  bool
}

// span is a version's place in its table: the first n entries, holding
// len live keys.
type span struct {
	t   *table
	n   int32
	len int
}

func newSpan() span { return span{t: &table{index: map[string]int32{}}} }

// version is a map's or a set's view of a table.
type version struct {
	span
	// pairs caches the live keys and values in order, built on first use.
	pairs atomic.Pointer[[2][]Value]
}

// latest is the key's entry within the first n, or -1. The caller holds
// the lock.
func (t *table) latest(k string, n int32) int32 {
	p, ok := t.index[k]
	if !ok {
		return -1
	}
	for p >= n {
		p = t.entries[p].prev
		if p < 0 {
			return -1
		}
	}
	return p
}

// get returns the value for k, and whether the key is present.
func (v *span) get(k string) (Value, bool) {
	if v.len == 0 {
		return Value{}, false
	}
	v.t.mu.RLock()
	defer v.t.mu.RUnlock()
	p := v.t.latest(k, v.n)
	if p < 0 || v.t.entries[p].gone {
		return Value{}, false
	}
	return v.t.entries[p].val, true
}

// with returns the version after setting key (whose canonical form is k)
// to val, or deleting it. It grows the table when v is its tip, and starts
// a new table from v's live entries otherwise. A change that changes
// nothing returns v's table and length unchanged.
func (v *version) with(k string, key, val Value, del bool) span {
	t := v.t
	t.mu.Lock()
	if int(v.n) != len(t.entries) || len(t.entries) > 2*t.live+32 {
		t.mu.Unlock()
		keys, vals := v.keysAndValues()
		fresh := newSpan()
		for i, kk := range keys {
			fresh.appendLocked(Key(kk), kk, vals[i], false)
		}
		fresh.appendLocked(k, key, val, del)
		fresh.t.live = fresh.len
		return fresh
	}
	out := v.span
	out.appendLocked(k, key, val, del)
	t.live = out.len
	t.mu.Unlock()
	return out
}

// appendLocked appends the entry to v, which is its table's tip, with the
// table locked or not yet shared.
func (v *span) appendLocked(k string, key, val Value, del bool) {
	t := v.t
	prev := t.latest(k, v.n)
	present := prev >= 0 && !t.entries[prev].gone
	switch {
	case del && !present:
		return
	case !del && present && Same(t.entries[prev].val, val):
		return
	}
	pos := int32(len(t.entries))
	start := pos
	if present {
		start = t.entries[prev].start
	}
	t.entries = append(t.entries, entry{k: k, key: key, val: val, prev: prev, start: start, gone: del})
	t.index[k] = pos
	v.n = pos + 1
	switch {
	case del:
		v.len--
	case !present:
		v.len++
	}
}

// keysAndValues returns the live keys and values in order. The slices are
// the version's own and must not be changed.
func (v *version) keysAndValues() ([]Value, []Value) {
	if p := v.pairs.Load(); p != nil {
		return p[0], p[1]
	}
	keys := make([]Value, 0, v.len)
	vals := make([]Value, 0, v.len)
	v.t.mu.RLock()
	for i := int32(0); i < v.n; i++ {
		e := &v.t.entries[i]
		if e.start != i || e.gone {
			continue
		}
		p := v.t.latest(e.k, v.n)
		if l := &v.t.entries[p]; l.start == i && !l.gone {
			keys = append(keys, e.key)
			vals = append(vals, l.val)
		}
	}
	v.t.mu.RUnlock()
	v.pairs.Store(&[2][]Value{keys, vals})
	return keys, vals
}
