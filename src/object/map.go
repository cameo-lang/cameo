package object

import (
	"strconv"
	"strings"
)

// Map is an immutable ordered map: keys in insertion order, looked up by
// structural equality.
type Map struct {
	keys  []Value
	vals  []Value
	index map[string]int
}

// NewMap makes an empty map.
func NewMap() *Map {
	return &Map{index: map[string]int{}}
}

// MapOf builds a map from alternating keys and values; a later duplicate
// key replaces the earlier value in place.
func MapOf(pairs ...Value) *Map {
	m := NewMap()
	for i := 0; i+1 < len(pairs); i += 2 {
		m.set(pairs[i], pairs[i+1])
	}
	return m
}

// set is the one mutation, used while a map is being built.
func (m *Map) set(k, v Value) {
	key := Key(k)
	if i, ok := m.index[key]; ok {
		m.vals[i] = v
		return
	}
	m.index[key] = len(m.keys)
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}

// Builder accumulates entries for a new map.
type MapBuilder struct{ m *Map }

// NewMapBuilder starts a map.
func NewMapBuilder() *MapBuilder { return &MapBuilder{m: NewMap()} }

// Set adds or replaces an entry.
func (b *MapBuilder) Set(k, v Value) *MapBuilder { b.m.set(k, v); return b }

// Map finishes the map; the builder must not be used afterwards.
func (b *MapBuilder) Map() *Map { return b.m }

// Value finishes the map as a value.
func (b *MapBuilder) Value() Value { return MapValue(b.m) }

// Len returns the number of entries.
func (m *Map) Len() int { return len(m.keys) }

// Get returns the value for k, and false when absent.
func (m *Map) Get(k Value) (Value, bool) {
	i, ok := m.index[Key(k)]
	if !ok {
		return Value{}, false
	}
	return m.vals[i], true
}

// Has reports whether k is a key.
func (m *Map) Has(k Value) bool {
	_, ok := m.index[Key(k)]
	return ok
}

// Keys and Values return the entries in order. The slices are the map's own
// and must not be changed.
func (m *Map) Keys() []Value   { return m.keys }
func (m *Map) Values() []Value { return m.vals }

// Put returns a copy with k set to v.
func (m *Map) Put(k, v Value) *Map {
	out := m.clone()
	out.set(k, v)
	return out
}

// Merge returns a copy with every entry of other set.
func (m *Map) Merge(other *Map) *Map {
	out := m.clone()
	for i, k := range other.keys {
		out.set(k, other.vals[i])
	}
	return out
}

// Without returns a copy without the keys.
func (m *Map) Without(keys ...Value) *Map {
	drop := map[string]bool{}
	for _, k := range keys {
		drop[Key(k)] = true
	}
	out := NewMap()
	for i, k := range m.keys {
		if !drop[Key(k)] {
			out.set(k, m.vals[i])
		}
	}
	return out
}

func (m *Map) clone() *Map {
	out := &Map{
		keys:  append([]Value(nil), m.keys...),
		vals:  append([]Value(nil), m.vals...),
		index: make(map[string]int, len(m.index)),
	}
	for k, i := range m.index {
		out.index[k] = i
	}
	return out
}

// Set is an immutable set in insertion order.
type Set struct {
	elems []Value
	index map[string]bool
}

// NewSet makes a set of the elements, dropping duplicates.
func NewSet(elems ...Value) *Set {
	s := &Set{index: map[string]bool{}}
	for _, e := range elems {
		s.add(e)
	}
	return s
}

func (s *Set) add(e Value) {
	key := Key(e)
	if s.index[key] {
		return
	}
	s.index[key] = true
	s.elems = append(s.elems, e)
}

// Len returns the number of elements.
func (s *Set) Len() int { return len(s.elems) }

// Has reports whether e is a member.
func (s *Set) Has(e Value) bool { return s.index[Key(e)] }

// Elems returns the members in order; the slice is the set's own.
func (s *Set) Elems() []Value { return s.elems }

// Union, Intersection and Difference return new sets.
func (s *Set) Union(o *Set) *Set {
	out := NewSet(s.elems...)
	for _, e := range o.elems {
		out.add(e)
	}
	return out
}

func (s *Set) Intersection(o *Set) *Set {
	out := NewSet()
	for _, e := range s.elems {
		if o.Has(e) {
			out.add(e)
		}
	}
	return out
}

func (s *Set) Difference(o *Set) *Set {
	out := NewSet()
	for _, e := range s.elems {
		if !o.Has(e) {
			out.add(e)
		}
	}
	return out
}

// Key is the canonical form of a value as a map key or set member: two
// structurally equal values have equal keys. An Integer and a Float of the
// same magnitude are different keys, as in Ruby.
func Key(v Value) string {
	var b strings.Builder
	writeKey(&b, v)
	return b.String()
}

func writeKey(b *strings.Builder, v Value) {
	switch v.kind {
	case KindNone, KindNil:
		b.WriteString("n")
	case KindBool:
		if v.n != 0 {
			b.WriteString("t")
		} else {
			b.WriteString("f")
		}
	case KindInt:
		b.WriteString("i")
		b.WriteString(strconv.FormatInt(v.n, 10))
	case KindFloat:
		b.WriteString("d")
		b.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 64))
	case KindString, KindSymbol:
		if v.kind == KindString {
			b.WriteString("s")
		} else {
			b.WriteString("y")
		}
		b.WriteString(strconv.Itoa(int(v.n)))
		b.WriteByte(':')
		b.WriteString(v.Str())
	case KindBytes:
		b.WriteString("b")
		b.WriteString(strconv.Itoa(int(v.n)))
		b.WriteByte(':')
		b.Write(v.Bytes())
	case KindList:
		b.WriteString("[")
		for _, e := range v.List().Elems {
			writeKey(b, e)
			b.WriteByte(',')
		}
		b.WriteString("]")
	case KindMap:
		// A map's identity does not depend on insertion order.
		m := v.Map()
		entries := make([]string, 0, len(m.keys))
		for i, k := range m.keys {
			entries = append(entries, Key(k)+">"+Key(m.vals[i]))
		}
		sortStrings(entries)
		b.WriteString("{")
		for _, e := range entries {
			b.WriteString(e)
			b.WriteByte(',')
		}
		b.WriteString("}")
	case KindSet:
		// A set's identity does not depend on insertion order.
		s := v.Set()
		keys := make([]string, 0, len(s.elems))
		for _, e := range s.elems {
			keys = append(keys, Key(e))
		}
		sortStrings(keys)
		b.WriteString("<")
		for _, k := range keys {
			b.WriteString(k)
			b.WriteByte(',')
		}
		b.WriteString(">")
	case KindRange:
		r := v.Range()
		b.WriteString("r")
		writeKey(b, r.Lo)
		if r.Exclusive {
			b.WriteString("...")
		} else {
			b.WriteString("..")
		}
		writeKey(b, r.Hi)
	case KindInstance:
		i := v.Instance()
		b.WriteString("o")
		b.WriteString(i.Cls.QualifiedName())
		b.WriteByte('#')
		b.WriteString(i.Cls.Digest)
		b.WriteString("(")
		for _, f := range i.Fields {
			writeKey(b, f)
			b.WriteByte(',')
		}
		b.WriteString(")")
	case KindClass:
		c := (*Class)(v.p)
		b.WriteString("c")
		b.WriteString(c.QualifiedName())
		b.WriteByte('#')
		b.WriteString(c.Digest)
	case KindTime:
		// Two times are equal when they are the same instant, whatever
		// their offsets.
		t := v.Time()
		b.WriteString("t")
		b.WriteString(strconv.FormatInt(t.Unix(), 10))
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(t.Nanosecond()))
	case KindException:
		e := v.Exception()
		b.WriteString("e")
		b.WriteString(e.Cls.QualifiedName())
		b.WriteByte(':')
		b.WriteString(e.Message)
	default:
		// Procs and processes have identity, not structure.
		b.WriteString("p")
		b.WriteString(strconv.FormatUint(uint64(identity(v)), 16))
	}
}

// OrNil returns v, or nil for the zero Value.
func OrNil(v Value) Value {
	if v.kind == KindNone {
		return Nil
	}
	return v
}

// Equal is structural equality for every builtin kind. For instances it
// compares class and fields; user-defined == is the evaluator's business.
// An integer and a float compare by magnitude.
func Equal(a, b Value) bool {
	if a.kind == KindInt || a.kind == KindFloat {
		if b.kind != KindInt && b.kind != KindFloat {
			return false
		}
		if a.kind == KindInt && b.kind == KindInt {
			return a.n == b.n
		}
		x, _ := a.AsNumber()
		y, _ := b.AsNumber()
		return x == y
	}
	if a.kind != b.kind || a.Class() != b.Class() {
		return false
	}
	return Key(a) == Key(b)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
