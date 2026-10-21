// Package object is cameo's value model: the kinds of value a script
// handles, their classes, structural equality and hashing, and the ordered
// Map and Set. Every value is immutable; a List, Map or Set is built once and
// never changed, so sharing one is always safe.
package object

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// Kind is what a Value holds.
type Kind uint8

const (
	// KindNone is the zero Value: no value at all, where an absent default,
	// an unset field or a missing range bound is meant. It is never a
	// script's value; cameo's nil is KindNil.
	KindNone Kind = iota
	KindNil
	KindBool
	KindInt
	KindFloat
	// KindString, KindBytes and KindASCII are a String in its three
	// encodings: UTF-8, ASCII-8BIT (binary) and US-ASCII. All three are
	// Ruby's String; the kind is the encoding.
	KindString
	KindSymbol
	KindBytes
	KindList
	KindMap
	KindSet
	KindRange
	KindInstance
	KindClass
	KindException
	// KindTime is an instant with its UTC offset, Ruby's Time.
	KindTime
	// KindRegexp is a compiled regular expression, KindMatch a match of
	// one: Ruby's Regexp and MatchData.
	KindRegexp
	KindMatch
	// KindBig is an Integer beyond 64 bits; see BigInt.
	KindBig
	// KindEnumerator is an Enumerator or an Enumerator::Lazy.
	KindEnumerator
	KindASCII
	// KindProc is the evaluator's: a block or lambda. The object is opaque
	// here; see Opaque.
	KindProc
)

func (k Kind) String() string {
	switch k {
	case KindNone:
		return "none"
	case KindNil:
		return "nil"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindString:
		return "string"
	case KindSymbol:
		return "symbol"
	case KindBytes:
		return "bytes"
	case KindList:
		return "list"
	case KindMap:
		return "map"
	case KindSet:
		return "set"
	case KindRange:
		return "range"
	case KindInstance:
		return "instance"
	case KindClass:
		return "class"
	case KindException:
		return "exception"
	case KindTime:
		return "time"
	case KindRegexp:
		return "regexp"
	case KindMatch:
		return "match"
	case KindBig:
		return "int"
	case KindEnumerator:
		return "enumerator"
	case KindASCII:
		return "ascii"
	case KindProc:
		return "proc"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// Value is one cameo value: a kind, an integer payload and a reference,
// three words that are passed and stored by value. An integer, a float, a
// boolean and nil live entirely in the payload and never touch the heap; a
// string or a symbol points at its bytes; every other kind points at its
// object. A Value is not comparable with ==, since two equal strings may
// point at different bytes: use Equal for structural equality, Same for
// identity, IsNil and IsZero for the two kinds of nothing.
type Value struct {
	_    [0]func()
	kind Kind
	n    int64
	p    unsafe.Pointer
}

// Nil is cameo's nil; True and False its booleans.
var (
	Nil   = Value{kind: KindNil}
	True  = Value{kind: KindBool, n: 1}
	False = Value{kind: KindBool, n: 0}
)

// Kind returns what the value holds.
func (v Value) Kind() Kind { return v.kind }

// IsZero reports the zero Value: no value at all.
func (v Value) IsZero() bool { return v.kind == KindNone }

// IsNil reports cameo's nil.
func (v Value) IsNil() bool { return v.kind == KindNil }

// Truthy reports whether a value counts as true: everything but nil and
// false. The zero Value counts as nil.
func (v Value) Truthy() bool {
	switch v.kind {
	case KindNone, KindNil:
		return false
	case KindBool:
		return v.n != 0
	}
	return true
}

// Class returns the class the value dispatches on.
func (v Value) Class() *Class {
	switch v.kind {
	case KindNone, KindNil:
		return NilClass
	case KindBool:
		return BoolClass
	case KindInt, KindBig:
		return IntegerClass
	case KindFloat:
		return FloatClass
	case KindString, KindBytes, KindASCII:
		return StringClass
	case KindSymbol:
		return SymbolClass
	case KindList:
		return ListClass
	case KindMap:
		return MapClass
	case KindSet:
		return SetClass
	case KindRange:
		return RangeClass
	case KindInstance:
		return (*Instance)(v.p).Cls
	case KindClass:
		return (*Class)(v.p).Class()
	case KindException:
		return (*Exception)(v.p).Cls
	case KindTime:
		return TimeClass
	case KindRegexp:
		return RegexpClass
	case KindMatch:
		return MatchDataClass
	case KindEnumerator:
		if (*Enumerator)(v.p).Lazy {
			return LazyClass
		}
		return EnumeratorClass
	case KindProc:
		return ProcClass
	}
	return NilClass
}

// Constructors.

// integer is any Go integer type, which Int converts.
type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Int makes an integer value.
func Int[T integer](i T) Value { return Value{kind: KindInt, n: int64(i)} }

// Float makes a float value.
func Float(f float64) Value { return Value{kind: KindFloat, n: int64(math.Float64bits(f))} }

// Bool makes a boolean value.
func Bool(b bool) Value {
	if b {
		return True
	}
	return False
}

// String makes a string value sharing s's bytes.
func String(s string) Value {
	return Value{kind: KindString, n: int64(len(s)), p: unsafe.Pointer(unsafe.StringData(s))}
}

// Symbol makes a symbol value sharing the name's bytes.
func Symbol(name string) Value {
	return Value{kind: KindSymbol, n: int64(len(name)), p: unsafe.Pointer(unsafe.StringData(name))}
}

// Bytes makes a binary (ASCII-8BIT) string sharing b, which must not be
// changed afterwards.
func Bytes(b []byte) Value {
	return Value{kind: KindBytes, n: int64(len(b)), p: unsafe.Pointer(unsafe.SliceData(b))}
}

// StringOf makes a string of s in the encoding kind names: KindString
// (UTF-8), KindBytes (binary) or KindASCII (US-ASCII).
func StringOf(kind Kind, s string) Value {
	return Value{kind: kind, n: int64(len(s)), p: unsafe.Pointer(unsafe.StringData(s))}
}

// IsString reports a String in any encoding.
func (v Value) IsString() bool {
	return v.kind == KindString || v.kind == KindBytes || v.kind == KindASCII
}

// ASCIIOnly reports a string of 7-bit bytes alone, which reads the same in
// every encoding.
func ASCIIOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// ListValue, MapValue, SetValue, RangeValue, InstanceValue, ClassValue and
// ExceptionValue wrap an object as a value.
func ListValue(l *List) Value           { return Value{kind: KindList, p: unsafe.Pointer(l)} }
func MapValue(m *Map) Value             { return Value{kind: KindMap, p: unsafe.Pointer(m)} }
func SetValue(s *Set) Value             { return Value{kind: KindSet, p: unsafe.Pointer(s)} }
func RangeValue(r *Range) Value         { return Value{kind: KindRange, p: unsafe.Pointer(r)} }
func InstanceValue(i *Instance) Value   { return Value{kind: KindInstance, p: unsafe.Pointer(i)} }
func ClassValue(c *Class) Value         { return Value{kind: KindClass, p: unsafe.Pointer(c)} }
func ExceptionValue(e *Exception) Value { return Value{kind: KindException, p: unsafe.Pointer(e)} }

// TimeValue makes a time value. Its location says how it shows: time.UTC
// for a UTC time, time.Local for the local zone, and a fixed zone for an
// offset.
func TimeValue(t time.Time) Value { return Value{kind: KindTime, p: unsafe.Pointer(&t)} }

// RegexpValue and MatchValue wrap a regexp and a match.
func RegexpValue(r *Regexp) Value { return Value{kind: KindRegexp, p: unsafe.Pointer(r)} }
func MatchValue(m *Match) Value   { return Value{kind: KindMatch, p: unsafe.Pointer(m)} }

// Opaque wraps an object this package does not know, a proc or a process,
// under its kind. The evaluator reads it back with Opaque.
func Opaque(kind Kind, p unsafe.Pointer) Value { return Value{kind: kind, p: p} }

// Readers. The unchecked ones (Int, Float, Str, ...) are for use after a
// kind check and panic on another kind; the As* ones report the kind.

func (v Value) Int() int64 {
	v.must(KindInt)
	return v.n
}

func (v Value) Float() float64 {
	v.must(KindFloat)
	return math.Float64frombits(uint64(v.n))
}

func (v Value) Bool() bool {
	v.must(KindBool)
	return v.n != 0
}

// Str returns the text of a string, in any encoding, or a symbol.
func (v Value) Str() string {
	if !v.IsString() && v.kind != KindSymbol {
		v.must(KindString)
	}
	if v.n == 0 {
		return ""
	}
	return unsafe.String((*byte)(v.p), int(v.n))
}

// Bytes returns the bytes of a byte string; the slice must not be changed.
func (v Value) Bytes() []byte {
	v.must(KindBytes)
	if v.n == 0 {
		return []byte{}
	}
	return unsafe.Slice((*byte)(v.p), int(v.n))
}

func (v Value) List() *List {
	v.must(KindList)
	return (*List)(v.p)
}

func (v Value) Map() *Map {
	v.must(KindMap)
	return (*Map)(v.p)
}

func (v Value) Set() *Set {
	v.must(KindSet)
	return (*Set)(v.p)
}

func (v Value) Range() *Range {
	v.must(KindRange)
	return (*Range)(v.p)
}

func (v Value) Instance() *Instance {
	v.must(KindInstance)
	return (*Instance)(v.p)
}

func (v Value) Exception() *Exception {
	v.must(KindException)
	return (*Exception)(v.p)
}

func (v Value) Time() time.Time {
	v.must(KindTime)
	return *(*time.Time)(v.p)
}

// Opaque returns the object of a proc or a process.
func (v Value) Opaque() unsafe.Pointer { return v.p }

func (v Value) must(k Kind) {
	if v.kind != k {
		panic(fmt.Sprintf("object: %s read as %s", v.kind, k))
	}
}

func (v Value) AsInt() (int64, bool) {
	if v.kind != KindInt {
		return 0, false
	}
	return v.n, true
}

func (v Value) AsFloat() (float64, bool) {
	if v.kind != KindFloat {
		return 0, false
	}
	return math.Float64frombits(uint64(v.n)), true
}

// AsNumber reads an integer or a float as a float.
func (v Value) AsNumber() (float64, bool) {
	switch v.kind {
	case KindInt:
		return float64(v.n), true
	case KindBig:
		return BigToFloat(v.Big()), true
	case KindFloat:
		return math.Float64frombits(uint64(v.n)), true
	}
	return 0, false
}

func (v Value) AsBool() (bool, bool) {
	if v.kind != KindBool {
		return false, false
	}
	return v.n != 0, true
}

// AsString reads a String in any encoding as its bytes.
func (v Value) AsString() (string, bool) {
	if !v.IsString() {
		return "", false
	}
	return v.Str(), true
}

func (v Value) AsSymbol() (string, bool) {
	if v.kind != KindSymbol {
		return "", false
	}
	return v.Str(), true
}

func (v Value) AsBytes() ([]byte, bool) {
	if v.kind != KindBytes {
		return nil, false
	}
	return v.Bytes(), true
}

func (v Value) AsList() (*List, bool) {
	if v.kind != KindList {
		return nil, false
	}
	return (*List)(v.p), true
}

func (v Value) AsMap() (*Map, bool) {
	if v.kind != KindMap {
		return nil, false
	}
	return (*Map)(v.p), true
}

func (v Value) AsSet() (*Set, bool) {
	if v.kind != KindSet {
		return nil, false
	}
	return (*Set)(v.p), true
}

func (v Value) AsRange() (*Range, bool) {
	if v.kind != KindRange {
		return nil, false
	}
	return (*Range)(v.p), true
}

func (v Value) Regexp() *Regexp {
	v.must(KindRegexp)
	return (*Regexp)(v.p)
}

func (v Value) AsRegexp() (*Regexp, bool) {
	if v.kind != KindRegexp {
		return nil, false
	}
	return (*Regexp)(v.p), true
}

func (v Value) Match() *Match {
	v.must(KindMatch)
	return (*Match)(v.p)
}

func (v Value) AsTime() (time.Time, bool) {
	if v.kind != KindTime {
		return time.Time{}, false
	}
	return *(*time.Time)(v.p), true
}

func (v Value) AsInstance() (*Instance, bool) {
	if v.kind != KindInstance {
		return nil, false
	}
	return (*Instance)(v.p), true
}

func (v Value) AsClass() (*Class, bool) {
	if v.kind != KindClass {
		return nil, false
	}
	return (*Class)(v.p), true
}

func (v Value) AsException() (*Exception, bool) {
	if v.kind != KindException {
		return nil, false
	}
	return (*Exception)(v.p), true
}

// Same reports identity: the same kind and the same payload. Two strings
// are the same only when they share their bytes.
func Same(a, b Value) bool { return a.kind == b.kind && a.n == b.n && a.p == b.p }

// Objects.

// List is an immutable sequence. Elems is never changed after construction.
type List struct {
	Elems []Value
	// kinds caches Kinds; see there.
	kinds uint32
}

// NewList makes a list of elems; the slice is owned by the list afterwards.
func NewList(elems ...Value) Value {
	if elems == nil {
		elems = []Value{}
	}
	return ListValue(&List{Elems: elems})
}

// Len returns the number of elements.
func (l *List) Len() int { return len(l.Elems) }

// Range is `lo..hi` or `lo...hi`; a zero Lo or Hi is a beginless or endless
// range.
type Range struct {
	Lo, Hi    Value
	Exclusive bool
}

// Instance is an instance of a user class: its fields, in declaration order.
// A zero field is unset and reads as nil. Message caches the protobuf
// message the instance is, once built; the fields never change, so it
// stays valid.
type Instance struct {
	Cls     *Class
	Fields  []Value
	Message any
}

// Field returns the field named name, and false when the class has none.
func (i *Instance) Field(name string) (Value, bool) {
	idx, ok := i.Cls.FieldIndex[name]
	if !ok {
		return Value{}, false
	}
	return i.Fields[idx], true
}

// Exception is a raised error: an instance of an exception class with a
// message. It travels up the evaluator as a Go error, and is a Value when
// a script holds it (`rescue => e`).
type Exception struct {
	Cls     *Class
	Message string
	// Line is the source line the exception was raised on, or 0.
	Line int
	// Attrs holds what an exception of Ruby's core carries besides its
	// message, by the name of the method that reads it: a NameError's name
	// and receiver, a NoMethodError's args, a KeyError's key and receiver.
	// It is nil when there is none, and never changed once the exception is
	// made.
	Attrs map[string]Value
}

// Attr returns the attribute name of the exception, and false when it has
// none.
func (e *Exception) Attr(name string) (Value, bool) {
	v, ok := e.Attrs[name]
	return v, ok
}

func (e *Exception) Error() string { return e.Cls.Name + ": " + e.Message }

// FormatFloat prints a float the way Ruby's Float#to_s does: an integral
// value keeps one decimal, and large and small magnitudes use an exponent.
func FormatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case math.IsNaN(f):
		return "NaN"
	}
	abs := math.Abs(f)
	if abs != 0 && (abs >= 1e15 || abs < 1e-4) {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		if len(exp) == 2 && (exp[0] == '+' || exp[0] == '-') {
			exp = exp[:1] + "0" + exp[1:]
		}
		return mant + "e" + exp
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// QuoteString prints a string the way Ruby's String#inspect does: the
// usual backslash escapes, "#" escaped where it would start an
// interpolation, a character Ruby does not print as \uXXXX, and a byte
// that is not UTF-8 as \xXX.
func QuoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02X`, s[i])
		case r == '#':
			if next := s[i+1:]; next != "" && (next[0] == '{' || next[0] == '$' || next[0] == '@') {
				b.WriteByte('\\')
			}
			b.WriteByte('#')
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case escapes[r] != "":
			b.WriteString(escapes[r])
		case !printable(r):
			if r < 0x10000 {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				fmt.Fprintf(&b, `\u{%X}`, r)
			}
		default:
			b.WriteRune(r)
		}
		i += size
	}
	b.WriteByte('"')
	return b.String()
}

// QuoteBytes prints bytes the way Ruby inspects a binary string: as
// QuoteString does, but byte by byte, with \xXX for each byte that is not
// printable ASCII.
func QuoteBytes(p []byte) string {
	var b strings.Builder
	b.WriteByte('"')
	for i, c := range p {
		r := rune(c)
		switch {
		case r == '#':
			if i+1 < len(p) && (p[i+1] == '{' || p[i+1] == '$' || p[i+1] == '@') {
				b.WriteByte('\\')
			}
			b.WriteByte('#')
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case escapes[r] != "":
			b.WriteString(escapes[r])
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02X`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

var escapes = map[rune]string{
	'\a': `\a`, '\b': `\b`, '\t': `\t`, '\n': `\n`, '\v': `\v`, '\f': `\f`, '\r': `\r`, '\x1b': `\e`,
}

// printable is Onigmo's print class, which decides what Ruby's inspect
// shows as it is: anything but controls, surrogates, unassigned codepoints
// and the line and paragraph separators.
func printable(r rune) bool {
	return unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Zs, unicode.Cf, unicode.Co)
}

// SymbolInspect prints a symbol the way Ruby's Symbol#inspect does: `:name`,
// quoted when the name is not a plain identifier or operator.
func SymbolInspect(s string) string {
	if isIdentifier(s) || isOperator(s) {
		return ":" + s
	}
	return ":" + QuoteString(s)
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f:
		case r >= '0' && r <= '9' && i > 0:
		case (r == '?' || r == '!' || r == '=') && i == len(s)-1 && i > 0:
		default:
			return false
		}
	}
	return true
}

var operators = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "**": true, "==": true, "!=": true,
	"<": true, ">": true, "<=": true, ">=": true, "<=>": true, "===": true, "[]": true, "[]=": true,
	"<<": true, ">>": true, "!": true, "&": true, "|": true, "^": true, "~": true, "+@": true, "-@": true, "=~": true,
}

func isOperator(s string) bool { return operators[s] }

// Cls returns the class a class value is.
func (v Value) Cls() *Class {
	v.must(KindClass)
	return (*Class)(v.p)
}
