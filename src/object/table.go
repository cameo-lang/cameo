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
	// index finds a key by its canonical form (Key), and words a key
	// that is one word: an integer, or a string or a symbol short enough
	// to be inline. A key is in one of the two, whichever its value
	// says (keyOf). Each is made when the first key goes into it.
	index map[string]int32
	words map[word]int32
	// hint is how many keys the table was made for, which the index
	// the first key goes into is made for.
	hint int
	// live counts the keys present at the tip, to know when the log is
	// mostly superseded entries and should be compacted.
	live int
}

// An entry sets a key to a value, or deletes it. prev is the key's
// previous entry and start the entry its current life began at: a key's
// place in the order is where it was last added after being absent.
type entry struct {
	k     tkey
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

// word is the key of a value that is one word: tag says what it is, an
// integer or the class of a string and its length, and bits is the
// integer or the bytes. Two values are the same key when their words
// are equal, so such a key is hashed and compared without its text.
type word struct{ tag, bits uint64 }

// tkey is a key of a table: a word, or the canonical form of a value
// that is none (Key).
type tkey struct {
	w word
	k string
}

// wordOf returns the word of v, and false when v is not one word.
func wordOf(v Value) (word, bool) {
	if v.p == mark(KindInt) {
		return word{tag: 'i', bits: uint64(v.n)}, true
	}
	d, ok := v.inline()
	if !ok {
		return word{}, false
	}
	// A string of ASCII alone is the same key in every encoding, as
	// writeKey has it.
	class := uint64('s')
	switch inlineKinds[d>>4] {
	case KindSymbol:
		class = 'y'
	case KindBytes:
		if uint64(v.n)&0x8080808080808080 != 0 {
			class = 'b'
		}
	case KindASCII:
		if uint64(v.n)&0x8080808080808080 != 0 {
			class = 'a'
		}
	}
	return word{tag: class<<8 | uint64(d&15), bits: uint64(v.n)}, true
}

// keyOf returns the key of v in a table.
func keyOf(v Value) tkey {
	if w, ok := wordOf(v); ok {
		return tkey{w: w}
	}
	return tkey{k: Key(v)}
}

// find returns the latest entry of the key k in the whole table.
func (t *table) find(k tkey) (int32, bool) {
	if k.w.tag != 0 {
		p, ok := t.words[k.w]
		return p, ok
	}
	p, ok := t.index[k.k]
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
func (t *table) latest(k tkey, n int32) int32 {
	p, ok := t.find(k)
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
func (v *span) get(k tkey) (Value, bool) {
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

// getBytes is get for a key in bytes, which a lookup need not make a
// string of.
func (v *span) getBytes(k []byte) (Value, bool) {
	if v.len == 0 {
		return Value{}, false
	}
	v.t.mu.RLock()
	defer v.t.mu.RUnlock()
	p, ok := v.t.index[string(k)]
	if !ok {
		return Value{}, false
	}
	for p >= v.n {
		if p = v.t.entries[p].prev; p < 0 {
			return Value{}, false
		}
	}
	if v.t.entries[p].gone {
		return Value{}, false
	}
	return v.t.entries[p].val, true
}

// with returns the version after setting key (whose canonical form is k)
// to val, or deleting it. It grows the table when v is its tip, and starts
// a new table from v's live entries otherwise. A change that changes
// nothing returns v's table and length unchanged.
func (v *version) with(k tkey, key, val Value, del bool) span {
	t := v.t
	t.mu.Lock()
	if int(v.n) != len(t.entries) || len(t.entries) > 2*t.live+32 {
		t.mu.Unlock()
		keys, vals := v.keysAndValues()
		fresh := newSpan()
		for i, kk := range keys {
			fresh.appendLocked(keyOf(kk), kk, vals[i], false)
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
func (v *span) appendLocked(k tkey, key, val Value, del bool) {
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
	switch {
	case k.w.tag != 0:
		if t.words == nil {
			t.words = make(map[word]int32, t.hint)
		}
		t.words[k.w] = pos
	default:
		if t.index == nil {
			t.index = make(map[string]int32, t.hint)
		}
		t.index[k.k] = pos
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
		p := v.t.latest(e.k, v.n)
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
