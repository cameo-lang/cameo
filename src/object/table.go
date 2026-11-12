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
	// index finds a key by its canonical form (Key), and ints a key
	// that is an integer by its number: such a key has no canonical
	// form made for it. Each is made when the first key goes into it,
	// for hint keys.
	index map[string]int32
	ints  map[int64]int32
	hint  int
	// live counts the keys present at the tip, to know when the log is
	// mostly superseded entries and should be compacted.
	live int
}

// An entry sets a key to a value, or deletes it. k is the canonical form
// of the key, empty for an integer. prev is the key's
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
	// head is a position no live key starts before, so finding the first
	// key, as Hash#shift does, need not scan the keys shifted already.
	head int32
}

func newSpan() span { return span{t: &table{}} }

// canon returns the canonical form of the key v as a table keeps it:
// none for an integer, which the table finds by its number.
func canon(v Value) string {
	if v.p == mark(KindInt) {
		return ""
	}
	return Key(v)
}

// find returns the latest entry in the whole table of the key whose
// canonical form is k.
func (t *table) find(k string, key Value) (int32, bool) {
	if key.p == mark(KindInt) {
		p, ok := t.ints[key.n]
		return p, ok
	}
	p, ok := t.index[k]
	return p, ok
}

// version is a map's or a set's view of a table.
type version struct {
	span
	// pairs caches the live keys and values in order, built on first use.
	pairs atomic.Pointer[[2][]Value]
}

// latest is the key's entry within the first n, or -1. The caller holds
// the lock.
func (t *table) latest(k string, key Value, n int32) int32 {
	p, ok := t.find(k, key)
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

// get returns the value for the key whose canonical form is k, and
// whether the key is present.
func (v *span) get(k string, key Value) (Value, bool) {
	if v.len == 0 {
		return Value{}, false
	}
	t := v.t
	t.mu.RLock()
	p := t.latest(k, key, v.n)
	if p < 0 || t.entries[p].gone {
		t.mu.RUnlock()
		return Value{}, false
	}
	val := t.entries[p].val
	t.mu.RUnlock()
	return val, true
}

// getBytes is get for a key in bytes, which a lookup need not make a
// string of.
func (v *span) getBytes(k []byte) (Value, bool) {
	if v.len == 0 {
		return Value{}, false
	}
	t := v.t
	t.mu.RLock()
	p, ok := t.index[string(k)]
	for ok && p >= v.n {
		p = t.entries[p].prev
		ok = p >= 0
	}
	if !ok || t.entries[p].gone {
		t.mu.RUnlock()
		return Value{}, false
	}
	val := t.entries[p].val
	t.mu.RUnlock()
	return val, true
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
			fresh.appendLocked(canon(kk), kk, vals[i], false)
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
	prev := t.latest(k, key, v.n)
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
	if key.p == mark(KindInt) {
		if t.ints == nil {
			t.ints = make(map[int64]int32, t.hint)
		}
		t.ints[key.n] = pos
	} else {
		if t.index == nil {
			t.index = make(map[string]int32, t.hint)
		}
		t.index[k] = pos
	}
	v.n = pos + 1
	switch {
	case del:
		v.len--
	case !present:
		v.len++
	}
}

// first returns the first live key, its value and the entry its life
// starts at, scanning from head.
func (v *span) first() (key, val Value, pos int32, ok bool) {
	v.t.mu.RLock()
	defer v.t.mu.RUnlock()
	for i := v.head; i < v.n; i++ {
		e := &v.t.entries[i]
		if e.start != i || e.gone {
			continue
		}
		p := v.t.latest(e.k, e.key, v.n)
		if l := &v.t.entries[p]; l.start == i && !l.gone {
			return e.key, l.val, i, true
		}
	}
	return Value{}, Value{}, 0, false
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
	for i := v.head; i < v.n; i++ {
		e := &v.t.entries[i]
		if e.start != i || e.gone {
			continue
		}
		p := v.t.latest(e.k, e.key, v.n)
		if l := &v.t.entries[p]; l.start == i && !l.gone {
			keys = append(keys, e.key)
			vals = append(vals, l.val)
		}
	}
	v.t.mu.RUnlock()
	v.pairs.Store(&[2][]Value{keys, vals})
	return keys, vals
}
