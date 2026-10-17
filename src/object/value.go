// Package object is amber's value model: the kinds of value a script
// handles, their classes, structural equality and hashing, and the ordered
// Map and Set. Every value is immutable; a List, Map or Set is built once and
// never changed, so sharing one is always safe.
package object

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

// Kind is what a Value holds.
type Kind uint8

const (
	// KindNone is the zero Value: no value at all, where an absent default,
	// an unset field or a missing range bound is meant. It is never a
	// script's value; amber's nil is KindNil.
	KindNone Kind = iota
	KindNil
	KindBool
	KindInt
	KindFloat
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
	// KindProc and KindProcess are the evaluator's: a block or lambda and
	// an async process. The object is opaque here; see Opaque.
	KindProc
	KindProcess
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
	case KindProc:
		return "proc"
	case KindProcess:
		return "process"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// Value is one amber value: a kind, an integer payload and a reference,
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

// Nil is amber's nil; True and False its booleans.
var (
	Nil   = Value{kind: KindNil}
	True  = Value{kind: KindBool, n: 1}
	False = Value{kind: KindBool, n: 0}
)

// Kind returns what the value holds.
func (v Value) Kind() Kind { return v.kind }

// IsZero reports the zero Value: no value at all.
func (v Value) IsZero() bool { return v.kind == KindNone }

// IsNil reports amber's nil.
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
	case KindInt:
		return IntegerClass
	case KindFloat:
		return FloatClass
	case KindString:
		return StringClass
	case KindSymbol:
		return SymbolClass
	case KindBytes:
		return BytesClass
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
	case KindProc:
		return ProcClass
	case KindProcess:
		return ProcessClass
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

// Bytes makes a byte string value sharing b, which must not be changed
// afterwards.
func Bytes(b []byte) Value {
	return Value{kind: KindBytes, n: int64(len(b)), p: unsafe.Pointer(unsafe.SliceData(b))}
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

// Str returns the text of a string or a symbol.
func (v Value) Str() string {
	if v.kind != KindString && v.kind != KindSymbol {
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

func (v Value) AsString() (string, bool) {
	if v.kind != KindString {
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
type List struct{ Elems []Value }

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
	if abs != 0 && (abs >= 1e16 || abs < 1e-4) {
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

// QuoteString prints a string the way Ruby's String#inspect does.
func QuoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\x1b':
			b.WriteString(`\e`)
		case '#':
			b.WriteByte('#')
		default:
			if r < 0x20 || r == 0x7f {
				h := strings.ToUpper(strconv.FormatInt(int64(r), 16))
				if len(h) == 1 {
					h = "0" + h
				}
				b.WriteString(`\x`)
				b.WriteString(h)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
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
