package object

import (
	"strconv"
	"strings"
)

// Map is an immutable ordered map: keys in insertion order, looked up by
// structural equality. It is a version of a table (table.go), so a copy
// with one more key costs O(1). A map may have a default, a value or a
// proc, for a key it does not hold; the copies Put, Merge and WithDefault
// make keep it, as Ruby's []=, merge and dup do, and a map built anew has
// none.
type Map struct {
	version
	// dflt is the default value and dfltProc the default proc, at most one
	// of them set.
	dflt, dfltProc Value
}

// NewMap makes an empty map.
func NewMap() *Map { return &Map{version: version{span: newSpan()}} }

func (m *Map) derive(s span) *Map {
	return &Map{version: version{span: s}, dflt: m.dflt, dfltProc: m.dfltProc}
}

// MapOf builds a map from alternating keys and values; a later duplicate
// key replaces the earlier value in place.
func MapOf(pairs ...Value) *Map {
	b := NewMapBuilder()
	for i := 0; i+1 < len(pairs); i += 2 {
		b.Set(pairs[i], pairs[i+1])
	}
	return b.Map()
}

// MapBuilder accumulates entries for a new map.
type MapBuilder struct{ s span }

// NewMapBuilder starts a map.
func NewMapBuilder() *MapBuilder { return &MapBuilder{s: newSpan()} }

// Set adds or replaces an entry.
func (b *MapBuilder) Set(k, v Value) *MapBuilder {
	b.s.appendLocked(Key(k), k, v, false)
	return b
}

// Map finishes the map; the builder must not be used afterwards.
func (b *MapBuilder) Map() *Map {
	b.s.t.live = b.s.len
	return &Map{version: version{span: b.s}}
}

// Value finishes the map as a value.
func (b *MapBuilder) Value() Value { return MapValue(b.Map()) }

// Len returns the number of entries.
func (m *Map) Len() int { return m.len }

// Get returns the value for k, and false when absent.
func (m *Map) Get(k Value) (Value, bool) { return m.get(Key(k)) }

// Has reports whether k is a key.
func (m *Map) Has(k Value) bool {
	_, ok := m.get(Key(k))
	return ok
}

// Keys and Values return the entries in order. The slices are the map's own
// and must not be changed.
func (m *Map) Keys() []Value {
	keys, _ := m.keysAndValues()
	return keys
}

func (m *Map) Values() []Value {
	_, vals := m.keysAndValues()
	return vals
}

// Default returns the default value and the default proc; each is zero
// when unset.
func (m *Map) Default() (value, proc Value) { return m.dflt, m.dfltProc }

// WithDefault returns a copy whose default is value or, when proc is set,
// proc.
func (m *Map) WithDefault(value, proc Value) *Map {
	return &Map{version: version{span: m.span}, dflt: value, dfltProc: proc}
}

// Put returns a copy with k set to v.
func (m *Map) Put(k, v Value) *Map { return m.derive(m.with(Key(k), k, v, false)) }

// Merge returns a copy with every entry of other set.
func (m *Map) Merge(other *Map) *Map {
	out := m
	keys, vals := other.keysAndValues()
	for i, k := range keys {
		out = out.Put(k, vals[i])
	}
	if out == m {
		return m.derive(m.span)
	}
	return out
}

// Shift returns the first entry and the map without it, keeping the
// default, as Hash#shift leaves it; ok is false for an empty map. A loop
// of shifts costs O(1) each.
func (m *Map) Shift() (key, val Value, rest *Map, ok bool) {
	key, val, pos, ok := m.first()
	if !ok {
		return Value{}, Value{}, m, false
	}
	s := m.with(Key(key), key, Value{}, true)
	if s.t == m.t {
		s.head = pos + 1
	}
	return key, val, m.derive(s), true
}

// Without returns a copy without the keys, and without a default, as
// Ruby's except returns.
func (m *Map) Without(keys ...Value) *Map {
	out := &Map{version: version{span: m.span}}
	for _, k := range keys {
		out = &Map{version: version{span: out.with(Key(k), k, Value{}, true)}}
	}
	return out
}

// Set is an immutable set in insertion order, a version of a table whose
// values are unused.
type Set struct {
	version
	// kinds caches Kinds; see there.
	kinds uint32
}

// NewSet makes a set of the elements, dropping duplicates.
func NewSet(elems ...Value) *Set {
	s := newSpan()
	for _, e := range elems {
		s.appendLocked(Key(e), e, True, false)
	}
	s.t.live = s.len
	return &Set{version: version{span: s}}
}

// Len returns the number of elements.
func (s *Set) Len() int { return s.len }

// Has reports whether e is a member.
func (s *Set) Has(e Value) bool {
	_, ok := s.get(Key(e))
	return ok
}

// Elems returns the members in order; the slice is the set's own.
func (s *Set) Elems() []Value {
	keys, _ := s.keysAndValues()
	return keys
}

// With and Without return the set with e added or removed, in O(1) when
// the set is the newest of its table.
func (s *Set) With(e Value) *Set { return &Set{version: version{span: s.with(Key(e), e, True, false)}} }
func (s *Set) Without(e Value) *Set {
	return &Set{version: version{span: s.with(Key(e), e, Value{}, true)}}
}

// Union, Intersection and Difference return new sets.
func (s *Set) Union(o *Set) *Set {
	out := s
	for _, e := range o.Elems() {
		out = out.With(e)
	}
	return out
}

func (s *Set) Intersection(o *Set) *Set {
	out := NewSet()
	for _, e := range s.Elems() {
		if o.Has(e) {
			out = out.With(e)
		}
	}
	return out
}

func (s *Set) Difference(o *Set) *Set {
	out := NewSet()
	for _, e := range s.Elems() {
		if !o.Has(e) {
			out = out.With(e)
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
	case KindBig:
		// A big integer never equals a KindInt, so the same prefix is safe.
		b.WriteString("i")
		b.WriteString(v.Big().String())
	case KindFloat:
		b.WriteString("d")
		f := v.Float()
		if f == 0 {
			f = 0 // -0.0 is eql? to 0.0 and hashes alike
		}
		b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
	case KindString, KindBytes, KindASCII, KindSymbol:
		// A string of ASCII alone is the same key in every encoding, as
		// in Ruby; one with other bytes is its encoding's.
		switch {
		case v.kind == KindSymbol:
			b.WriteString("y")
		case v.kind == KindString || ASCIIOnly(v.Str()):
			b.WriteString("s")
		case v.kind == KindBytes:
			b.WriteString("b")
		default:
			b.WriteString("a")
		}
		b.WriteString(strconv.Itoa(int(v.n)))
		b.WriteByte(':')
		b.WriteString(v.Str())
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
		keys, vals := m.keysAndValues()
		entries := make([]string, 0, len(keys))
		for i, k := range keys {
			entries = append(entries, Key(k)+">"+Key(vals[i]))
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
		keys := make([]string, 0, s.Len())
		for _, e := range s.Elems() {
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
	case KindRegexp:
		r := v.Regexp()
		b.WriteString("x")
		// The n flag changes nothing for an ASCII pattern, and Ruby's ==
		// and hash leave it out.
		options := r.Options
		if allASCII(r.Source) {
			options &^= RegexpNoEncoding
		}
		b.WriteString(strconv.Itoa(options))
		b.WriteByte(':')
		b.WriteString(r.Source)
	case KindMatch:
		b.WriteString("m")
		b.WriteString(v.Match().key())
	case KindException:
		e := v.Exception()
		b.WriteString("e")
		b.WriteString(e.Cls.QualifiedName())
		b.WriteByte('#')
		b.WriteString(e.Cls.Digest)
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(len(e.Message)))
		b.WriteByte(':')
		b.WriteString(e.Message)
		for _, f := range e.Fields {
			b.WriteByte(',')
			writeKey(b, f)
		}
	case KindEnumerator:
		if a := v.Enumerator().Arith; a != nil {
			// An arithmetic sequence is its numbers, however it was made.
			b.WriteString("A")
			writeKey(b, a.Begin)
			b.WriteByte(',')
			writeKey(b, a.End)
			b.WriteByte(',')
			writeKey(b, a.Step)
			if a.Excl {
				b.WriteString("...")
			}
			return
		}
		b.WriteString("p")
		b.WriteString(strconv.FormatUint(uint64(identity(v)), 16))
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
	if a.isNumber() {
		if !b.isNumber() {
			return false
		}
		if a.kind == KindInt && b.kind == KindInt {
			return a.n == b.n
		}
		r, ok := compareNumbers(a, b)
		return ok && r == 0
	}
	if a.IsString() && b.IsString() {
		return Key(a) == Key(b)
	}
	if a.kind != b.kind || a.Class() != b.Class() {
		return false
	}
	if a.kind == KindEnumerator {
		// Arithmetic sequences compare their numbers with ==, as Ruby's
		// do, while their Key, which hash and eql? read, tells 1 and 1.0
		// apart.
		x, y := a.Enumerator().Arith, b.Enumerator().Arith
		if x != nil && y != nil {
			return x.Excl == y.Excl && Equal(x.Begin, y.Begin) && Equal(x.End, y.End) && Equal(x.Step, y.Step)
		}
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
