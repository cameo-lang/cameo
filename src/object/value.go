// Package object is cameo's value model: the kinds of value a script
// handles, their classes, structural equality and hashing, and the ordered
// Map and Set. Every value is immutable; a List, Map or Set is built once and
// never changed, so sharing one is always safe.
package object

import (
	"fmt"
	"github.com/cameo-lang/cameo/src/internal/trusted"
	"github.com/cameo-lang/cameo/src/str"
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
	// KindRandom is a Random: a handle on a generator; see Random.
	KindRandom
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
	case KindRandom:
		return "random"
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
	_ [0]func()
	p unsafe.Pointer
	n int64
}

// A Value is two words. Its kind is not a field of its own:
//
//   - nil, a boolean, an integer and a float have their payload in n, all
//     64 bits of it, and p points at the mark of their kind, an element of
//     kindMarks: the address says the kind;
//   - a string or a symbol has its bytes at p, its length in bytes in the
//     low 32 bits of n and, in the 24 bits above them, how many of its
//     bytes are not the first of a character (extraUnset when nobody has
//     counted); one of no bytes has the mark of its kind at p;
//   - every other kind has its object at p;
//   - the top byte of n is the kind of whatever has no mark at p.
//
// p is a pointer Go's garbage collector follows, or the address of a
// mark, which is a global. The zero Value has neither, and is KindNone.
var kindMarks [256]byte

const (
	kindShift = 56
	// A string's n has two fields under its kind. The low sizeBits are
	// its length in bytes. The extra field above them is the length in
	// bytes less the length in characters, of a UTF-8 string whose
	// characters were counted where it was made: 0 for ASCII, whose
	// characters are its bytes. All ones, extraUnset, says nobody
	// counted: most strings are never asked. A Value has no room of its
	// own for the count, and a third word would make every value of
	// every kind half as big again, so the count shares n, and a string
	// whose difference does not fit the field is counted when asked.
	sizeBits   = 32
	sizeMask   = 1<<sizeBits - 1
	extraMask  = 1<<(kindShift-sizeBits) - 1
	extraUnset = extraMask
)

// mark returns the mark of kind k. A kind that lives in the payload is
// tested by comparing p with its mark, which is one comparison where
// kind is two.
func mark(k Kind) unsafe.Pointer { return unsafe.Pointer(&kindMarks[k]) }

// kind returns the kind of v.
func (v Value) kind() Kind {
	if d := uintptr(v.p) - uintptr(unsafe.Pointer(&kindMarks[0])); d < uintptr(len(kindMarks)) {
		return Kind(d)
	}
	return Kind(uint64(v.n) >> kindShift)
}

// immValue makes a value of a kind that lives in the payload.
func immValue(k Kind, n int64) Value { return Value{p: mark(k), n: n} }

// ptrValue makes a value of a kind that is an object, at p.
func ptrValue(k Kind, p unsafe.Pointer) Value { return Value{p: p, n: int64(k) << kindShift} }

// strValue makes a string or a symbol of n bytes at p.
func strValue(k Kind, n int64, p unsafe.Pointer) Value {
	if n == 0 {
		return Value{p: mark(k)}
	}
	if n > sizeMask {
		panic("object: a string of more than 4 GiB")
	}
	return Value{p: p, n: n | extraUnset<<sizeBits | int64(k)<<kindShift}
}

// size returns the number of bytes of a string or a symbol.
func (v Value) size() int { return int(v.n & sizeMask) }

// Nil is cameo's nil; True and False its booleans.
var (
	Nil   = immValue(KindNil, 0)
	True  = immValue(KindBool, 1)
	False = immValue(KindBool, 0)
)

// Kind returns what the value holds.
func (v Value) Kind() Kind { return v.kind() }

// IsZero returns true for the zero Value: no value at all.
func (v Value) IsZero() bool { return v.p == nil && v.n == 0 }

// IsNil returns true for cameo's nil.
func (v Value) IsNil() bool { return v.p == mark(KindNil) }

// Truthy returns whether a value counts as true: everything but nil and
// false. The zero Value counts as nil.
func (v Value) Truthy() bool {
	if v.p == mark(KindBool) {
		return v.n != 0
	}
	return v.p != mark(KindNil) && !v.IsZero()
}

// Class returns the class the value dispatches on.
func (v Value) Class() *Class {
	switch v.kind() {
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
		switch e := (*Enumerator)(v.p); {
		case e.Lazy:
			return LazyClass
		case e.Arith != nil:
			return ArithSeqClass
		case e.Of != nil:
			return e.Of
		}
		return EnumeratorClass
	case KindProc:
		return ProcClass
	case KindRandom:
		return RandomClass
	}
	return NilClass
}

// Constructors.

// integer is any Go integer type, which Int converts.
type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Int makes an integer value.
func Int[T integer](i T) Value { return immValue(KindInt, int64(i)) }

// Float makes a float value.
func Float(f float64) Value { return immValue(KindFloat, int64(math.Float64bits(f))) }

// Bool makes a boolean value.
func Bool(b bool) Value {
	if b {
		return True
	}
	return False
}

// WrongEncoding is what NewString panics with for bytes that are not
// what the encoding given says. The evaluator raises it in the script
// as an ArgumentError.
type WrongEncoding struct {
	Encoding str.Encoding
	Bytes    string
}

func (e *WrongEncoding) Error() string {
	name := "UTF-8"
	switch e.Encoding {
	case str.ASCII:
		name = "ASCII"
	case str.USASCII:
		name = "US-ASCII"
	}
	shown := e.Bytes
	if len(shown) > 40 {
		shown = shown[:40]
	}
	return "invalid byte sequence in " + name + ": " + strconv.Quote(shown) + " is not text; keep such bytes in a binary string"
}

// NewString makes a string value sharing s's bytes, which are what enc
// says. It reads the bytes given as ASCII, US-ASCII or UTF-8 to see
// that they are, and panics with a *WrongEncoding when they are not:
// no string says of itself what it is not. A caller that takes bytes
// from outside chooses: str.Binary for bytes, or str.UTF8 for text, through
// TryUTF8 when it has an error of its own for bytes that are none.
func NewString(s string, enc str.Encoding) Value {
	switch enc {
	case str.ASCII:
		if !str.ASCIIOnly(s) {
			panic(&WrongEncoding{enc, s})
		}
		return stringOfChars(s, len(s))
	case str.UTF8:
		chars, ok := str.Count(s)
		if !ok {
			panic(&WrongEncoding{enc, s})
		}
		return stringOfChars(s, chars)
	case str.USASCII:
		if !str.ASCIIOnly(s) {
			panic(&WrongEncoding{enc, s})
		}
		return strValue(KindASCII, int64(len(s)), unsafe.Pointer(unsafe.StringData(s)))
	case str.Binary:
		return strValue(KindBytes, int64(len(s)), unsafe.Pointer(unsafe.StringData(s)))
	}
	panic("object.NewString: no encoding")
}

// TryUTF8 makes the UTF-8 string of s, and returns false when the
// bytes are not UTF-8: for a caller that takes bytes from outside as
// text, a file or a host, and has its own error to raise for bytes
// that are none. One that takes them as bytes makes a str.Binary string.
func TryUTF8(s string) (Value, bool) {
	chars, ok := str.Count(s)
	if !ok {
		return Value{}, false
	}
	return stringOfChars(s, chars), true
}

// EncodingOf returns the encoding of the strings of kind k: KindString
// is str.UTF8, KindBytes str.Binary and KindASCII str.USASCII.
func EncodingOf(k Kind) str.Encoding {
	switch k {
	case KindBytes:
		return str.Binary
	case KindASCII:
		return str.USASCII
	}
	return str.UTF8
}

// KindOf returns the kind of the strings of the encoding enc: the
// reverse of EncodingOf, with ASCII a UTF-8 string as UTF8 is.
func KindOf(enc str.Encoding) Kind {
	switch enc {
	case str.Binary:
		return KindBytes
	case str.USASCII:
		return KindASCII
	}
	return KindString
}

// uncounted makes a UTF-8 string value that does not know its
// characters: what stringOfChars starts from.
func uncounted(s string) Value {
	return strValue(KindString, int64(len(s)), unsafe.Pointer(unsafe.StringData(s)))
}

// Symbol makes a symbol value sharing the name's bytes.
func Symbol(name string) Value {
	return strValue(KindSymbol, int64(len(name)), unsafe.Pointer(unsafe.StringData(name)))
}

// Bytes makes a binary (ASCII-8BIT) string sharing b, which must not be
// changed afterwards.
func Bytes(b []byte) Value {
	return strValue(KindBytes, int64(len(b)), unsafe.Pointer(unsafe.SliceData(b)))
}

// TrustedString makes a string of s in the encoding enc without
// reading s: for cameo's own code, which made the bytes from strings
// that were checked, a slice of one or two joined, and so knows what
// they are. A host cannot call it, since it cannot name trusted.Key;
// it makes its strings with NewString, which checks.
//
// chars is the number of characters of a str.UTF8 string, when the caller
// has it, and str.UnknownLength when not: the string is then counted when
// it is asked. For ASCII the characters are the bytes, and chars is
// not looked at; the other encodings have none.
//
// A caller that is wrong makes a string that says of itself what it
// is not, and whose characters are not where it says.
func TrustedString(_ trusted.Key, s string, enc str.Encoding, chars int) Value {
	switch enc {
	case str.ASCII:
		return stringOfChars(s, len(s))
	case str.UTF8:
		return stringOfChars(s, chars)
	case str.USASCII:
		return strValue(KindASCII, int64(len(s)), unsafe.Pointer(unsafe.StringData(s)))
	case str.Binary:
		return strValue(KindBytes, int64(len(s)), unsafe.Pointer(unsafe.StringData(s)))
	}
	panic("object.TrustedString: no encoding")
}

// stringOfChars makes a UTF-8 string of s with chars characters,
// str.UnknownLength for a string nobody counted. A count that does not fit
// beside the length in bytes is dropped, and the string is counted
// when asked.
func stringOfChars(s string, chars int) Value {
	// A count above the bytes or too far under them is dropped; the
	// comparison is unsigned to be one for the two. The string of no
	// bytes has no n to keep a count in, and needs none.
	v := uncounted(s)
	if extra := uint(len(s) - chars); extra < extraUnset && len(s) > 0 {
		v.n ^= int64(extra^extraUnset) << sizeBits
	}
	return v
}

// Chars returns the number of characters of a UTF-8 string that knows
// it, as every one does that was counted and whose count fits beside
// its length. It returns str.UnknownLength for any other: a string in
// another encoding, one too long to keep its count, one made without.
func (v Value) Chars() int {
	if v.p == mark(KindString) {
		return 0
	}
	extra := int(v.n >> sizeBits & extraMask)
	if extra == extraUnset || v.kind() != KindString {
		return str.UnknownLength
	}
	return v.size() - extra
}

// KnownASCII returns true for a UTF-8 string that is known to hold ASCII
// alone: one whose characters are known to be its bytes. False says
// nothing: the string may still be ASCII.
func (v Value) KnownASCII() bool {
	return v.Chars() == v.size()
}

// Joined makes the UTF-8 string s, which is a and then b: a string that
// knows its characters when both of them do. A b that begins inside a
// character is left out, since it may complete a character a ends in
// the middle of, and the two then have one character less together.
func Joined(s string, a, b Value) Value {
	an, bn := a.Chars(), b.Chars()
	if an < 0 || bn < 0 || bn > 0 && !utf8.RuneStart(*(*byte)(b.p)) {
		return NewString(s, str.UTF8)
	}
	return stringOfChars(s, an+bn)
}

// IsString returns true for a String in any encoding.
func (v Value) IsString() bool {
	k := v.kind()
	return k == KindString || k == KindBytes || k == KindASCII
}

// ListValue, MapValue, SetValue, RangeValue, InstanceValue, ClassValue and
// ExceptionValue wrap an object as a value.
func ListValue(l *List) Value           { return ptrValue(KindList, unsafe.Pointer(l)) }
func MapValue(m *Map) Value             { return ptrValue(KindMap, unsafe.Pointer(m)) }
func SetValue(s *Set) Value             { return ptrValue(KindSet, unsafe.Pointer(s)) }
func RangeValue(r *Range) Value         { return ptrValue(KindRange, unsafe.Pointer(r)) }
func InstanceValue(i *Instance) Value   { return ptrValue(KindInstance, unsafe.Pointer(i)) }
func ClassValue(c *Class) Value         { return ptrValue(KindClass, unsafe.Pointer(c)) }
func ExceptionValue(e *Exception) Value { return ptrValue(KindException, unsafe.Pointer(e)) }

// TimeValue makes a time value. Its location says how it shows: time.UTC
// for a UTC time, time.Local for the local zone, a fixed zone for an
// offset, and a loaded IANA zone, whose offset follows the date, for a
// zone named by a script.
func TimeValue(t time.Time) Value { return ptrValue(KindTime, unsafe.Pointer(&t)) }

// RegexpValue and MatchValue wrap a regexp and a match.
func RegexpValue(r *Regexp) Value { return ptrValue(KindRegexp, unsafe.Pointer(r)) }
func MatchValue(m *Match) Value   { return ptrValue(KindMatch, unsafe.Pointer(m)) }

// Opaque wraps an object this package does not know, a proc or a process,
// under its kind. The evaluator reads it back with Opaque.
func Opaque(kind Kind, p unsafe.Pointer) Value { return ptrValue(kind, p) }

// Readers. The unchecked ones (Int, Float, Str, ...) are for use after a
// kind check and panic on another kind; the As* ones return whether the
// kind fits.

func (v Value) Int() int64 {
	if v.p != mark(KindInt) {
		v.must(KindInt)
	}
	return v.n
}

func (v Value) Float() float64 {
	if v.p != mark(KindFloat) {
		v.must(KindFloat)
	}
	return math.Float64frombits(uint64(v.n))
}

func (v Value) Bool() bool {
	if v.p != mark(KindBool) {
		v.must(KindBool)
	}
	return v.n != 0
}

// Str returns the text of a string, in any encoding, or a symbol.
func (v Value) Str() string {
	if k := v.kind(); k != KindString && k != KindSymbol && k != KindBytes && k != KindASCII {
		v.must(KindString)
	}
	n := v.size()
	if n == 0 {
		return ""
	}
	return unsafe.String((*byte)(v.p), n)
}

// Bytes returns the bytes of a byte string; the slice must not be changed.
func (v Value) Bytes() []byte {
	v.must(KindBytes)
	n := v.size()
	if n == 0 {
		return []byte{}
	}
	return unsafe.Slice((*byte)(v.p), n)
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
	if v.kind() != k {
		panic(fmt.Sprintf("object: %s read as %s", v.kind(), k))
	}
}

func (v Value) AsInt() (int64, bool) {
	if v.p != mark(KindInt) {
		return 0, false
	}
	return v.n, true
}

func (v Value) AsFloat() (float64, bool) {
	if v.p != mark(KindFloat) {
		return 0, false
	}
	return math.Float64frombits(uint64(v.n)), true
}

// AsNumber reads an integer or a float as a float.
func (v Value) AsNumber() (float64, bool) {
	switch v.kind() {
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
	if v.p != mark(KindBool) {
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
	if v.kind() != KindSymbol {
		return "", false
	}
	return v.Str(), true
}

// AsFieldName returns the field a keyword names, by a Symbol or a String,
// and false for any other key.
func (v Value) AsFieldName() (string, bool) {
	if v.kind() == KindSymbol || v.IsString() {
		return v.Str(), true
	}
	return "", false
}

func (v Value) AsBytes() ([]byte, bool) {
	if v.kind() != KindBytes {
		return nil, false
	}
	return v.Bytes(), true
}

func (v Value) AsList() (*List, bool) {
	if v.kind() != KindList {
		return nil, false
	}
	return (*List)(v.p), true
}

func (v Value) AsMap() (*Map, bool) {
	if v.kind() != KindMap {
		return nil, false
	}
	return (*Map)(v.p), true
}

func (v Value) AsSet() (*Set, bool) {
	if v.kind() != KindSet {
		return nil, false
	}
	return (*Set)(v.p), true
}

func (v Value) AsRange() (*Range, bool) {
	if v.kind() != KindRange {
		return nil, false
	}
	return (*Range)(v.p), true
}

func (v Value) Regexp() *Regexp {
	v.must(KindRegexp)
	return (*Regexp)(v.p)
}

func (v Value) AsRegexp() (*Regexp, bool) {
	if v.kind() != KindRegexp {
		return nil, false
	}
	return (*Regexp)(v.p), true
}

func (v Value) Match() *Match {
	v.must(KindMatch)
	return (*Match)(v.p)
}

func (v Value) AsTime() (time.Time, bool) {
	if v.kind() != KindTime {
		return time.Time{}, false
	}
	return *(*time.Time)(v.p), true
}

func (v Value) AsInstance() (*Instance, bool) {
	if v.kind() != KindInstance {
		return nil, false
	}
	return (*Instance)(v.p), true
}

func (v Value) AsClass() (*Class, bool) {
	if v.kind() != KindClass {
		return nil, false
	}
	return (*Class)(v.p), true
}

func (v Value) AsException() (*Exception, bool) {
	if v.kind() != KindException {
		return nil, false
	}
	return (*Exception)(v.p), true
}

// Same returns whether a and b are identical: the same kind and the same
// payload. Two strings are the same only when they share their bytes.
func Same(a, b Value) bool {
	if a.p != b.p {
		return false
	}
	// Two strings that share their bytes are the same whether or not
	// one of them knows its characters.
	return a.n == b.n || (a.n^b.n)&^(extraMask<<sizeBits) == 0 && a.IsString()
}

// Objects.

// List is an immutable sequence. Elems is never changed after construction,
// and has no capacity beyond its length when the list is grown.
type List struct {
	Elems []Value
	// kinds caches Kinds; see there.
	kinds uint32
	// grown is set for a list Append made, which is the List of a grown
	// (append.go).
	grown bool
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
// a script holds it (`rescue => e`). Like an Instance it is a protobuf
// message, whose parts protogen names: the message, the line, the cause,
// the Attrs, and the Fields the class declares.
type Exception struct {
	Cls     *Class
	Message string
	// Line is the source line the exception was raised on, or 0.
	Line int
	// File is the source the line is in when it is not the script's: a
	// library written in cameo. "" is the script, or not yet known.
	File string
	// Attrs holds what an exception of Ruby's core carries besides its
	// message, by the name of the method that reads it: a NameError's name
	// and receiver, a NoMethodError's args, a KeyError's key and receiver,
	// a SystemExit's status, an UncaughtThrowError's tag and value. It is
	// nil when there is none, and never changed once the exception is made.
	Attrs map[string]Value
	// Cause is the exception that was being rescued when this one was
	// raised: zero when there was none, nil when the raise said
	// `cause: nil`.
	Cause Value
	// Fields are the fields the class declares, in its declaration order,
	// as an Instance holds them; nil for a class that declares none.
	Fields []Value
	// Building is set while new runs initialize on the exception, the one
	// time its message and fields may be set.
	Building bool
}

// Attr returns the attribute name of the exception, and false when it has
// none.
func (e *Exception) Attr(name string) (Value, bool) {
	v, ok := e.Attrs[name]
	return v, ok
}

// Copy returns a copy of the exception to change before anyone else sees
// it: the copy shares the attributes and the fields, which are never
// changed in place.
func (e *Exception) Copy() *Exception {
	c := *e
	c.Building = false
	return &c
}

// Field returns the field named name, and false when the class has none.
func (e *Exception) Field(name string) (Value, bool) {
	idx, ok := e.Cls.FieldIndex[name]
	if !ok || idx >= len(e.Fields) {
		return Value{}, false
	}
	return e.Fields[idx], true
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
// quoted when the name is not one Ruby's parser reads after a colon.
func SymbolInspect(s string) string {
	if isSymbolName(s) {
		return ":" + s
	}
	return ":" + QuoteString(s)
}

// SymbolKey returns how Hash#inspect prints a symbol key, CRuby 3.4's
// `name: `, or `"name": ` where `name:` would not read back as the symbol:
// a name `:name` would quote, one that starts with @, $ or !, and one that
// ends in an operator character, as `foo=:` and `+:` do not parse.
func SymbolKey(s string) string {
	if !isSymbolName(s) || strings.ContainsAny(s[:1], "@$!") || strings.ContainsAny(s[len(s)-1:], "+-*/`%^&|]<=>~@") {
		return QuoteString(s) + ": "
	}
	return s + ": "
}

// isSymbolName returns whether Ruby's parser reads s after a colon: an
// identifier, a method name ending in ?, ! or =, an operator, or the name of
// an instance, class or global variable.
func isSymbolName(s string) bool {
	switch {
	case isMethodName(s) || operators[s]:
		return true
	case strings.HasPrefix(s, "@@"):
		return isIdentifier(s[2:])
	case strings.HasPrefix(s, "@"):
		return isIdentifier(s[1:])
	case strings.HasPrefix(s, "$"):
		return isGlobalName(s[1:])
	}
	return false
}

// isGlobalName returns whether s is what follows the $ of a global
// variable's name: an identifier, a match group ($1, $10, but not $01),
// $0, one punctuation character ($~, $!), or $- and one character ($-w).
func isGlobalName(s string) bool {
	switch {
	case isIdentifier(s):
		return true
	case len(s) == 1 && strings.ContainsAny(s, "~*$?!@/\\;,.=:<>\"&'`+0"):
		return true
	case len(s) == 2 && s[0] == '-':
		r := s[1]
		return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
	case s != "" && s[0] >= '1' && s[0] <= '9':
		return strings.Trim(s, "0123456789") == ""
	}
	return false
}

// isMethodName returns whether s is an identifier, or one followed by ?, !
// or =.
func isMethodName(s string) bool {
	if n := len(s); n > 1 && strings.ContainsAny(s[n-1:], "?!=") {
		s = s[:n-1]
	}
	return isIdentifier(s)
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 0x7f:
		case r >= '0' && r <= '9' && i > 0:
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
	"!~": true, "`": true,
}

// Cls returns the class a class value is.
func (v Value) Cls() *Class {
	v.must(KindClass)
	return (*Class)(v.p)
}

// NewSystemExit returns the exception exit raises: a SystemExit with the
// status the command ends with, which cameo.ExitStatus reads.
func NewSystemExit(status int64, msg string) *Exception {
	return &Exception{Cls: SystemExit, Message: msg, Attrs: map[string]Value{"status": Int(status)}}
}
