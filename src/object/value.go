// Package object is amber's value model: the kinds of value a script
// handles, their classes, structural equality and hashing, and the ordered
// Map and Set. Every value is immutable; a List, Map or Set is built once and
// never changed, so sharing one is always safe.
package object

import (
	"math"
	"strconv"
	"strings"
)

// Value is any amber value. Class answers the class the value dispatches on.
type Value interface {
	Class() *Class
}

// Nil is amber's nil; True and False its booleans.
var (
	Nil   Value = nilValue{}
	True  Value = Bool(true)
	False Value = Bool(false)
)

type nilValue struct{}

func (nilValue) Class() *Class { return NilClass }

// Bool is true or false.
type Bool bool

func (Bool) Class() *Class { return BoolClass }

// Int is a 64-bit integer.
type Int int64

func (Int) Class() *Class { return IntegerClass }

// Float is a 64-bit float.
type Float float64

func (Float) Class() *Class { return FloatClass }

// String is an immutable UTF-8 string.
type String string

func (String) Class() *Class { return StringClass }

// Symbol is a name: a method name, a map key, an enum value.
type Symbol string

func (Symbol) Class() *Class { return SymbolClass }

// Bytes is a byte string, the value of a bytes field.
type Bytes []byte

func (Bytes) Class() *Class { return BytesClass }

// List is an immutable sequence. Elems is never changed after construction.
type List struct{ Elems []Value }

func (*List) Class() *Class { return ListClass }

// NewList makes a list of elems; the slice is owned by the list afterwards.
func NewList(elems ...Value) *List {
	if elems == nil {
		elems = []Value{}
	}
	return &List{Elems: elems}
}

// Len returns the number of elements.
func (l *List) Len() int { return len(l.Elems) }

// Range is `lo..hi` or `lo...hi`; a nil Lo or Hi is a beginless or endless
// range.
type Range struct {
	Lo, Hi    Value
	Exclusive bool
}

func (*Range) Class() *Class { return RangeClass }

// Instance is an instance of a user class: its fields, in declaration order.
// Message caches the protobuf message the instance is, once built; the
// fields never change, so it stays valid.
type Instance struct {
	Cls     *Class
	Fields  []Value
	Message any
}

func (i *Instance) Class() *Class { return i.Cls }

// Field returns the field named name, and false when the class has none.
func (i *Instance) Field(name string) (Value, bool) {
	idx, ok := i.Cls.FieldIndex[name]
	if !ok {
		return nil, false
	}
	return i.Fields[idx], true
}

// Exception is a raised error: an instance of an exception class with a
// message.
type Exception struct {
	Cls     *Class
	Message string
	// Line is the source line the exception was raised on, or 0.
	Line int
}

func (e *Exception) Class() *Class { return e.Cls }

func (e *Exception) Error() string { return e.Cls.Name + ": " + e.Message }

// Truthy reports whether a value counts as true: everything but nil and false.
func Truthy(v Value) bool {
	switch v := v.(type) {
	case nilValue:
		return false
	case Bool:
		return bool(v)
	}
	return true
}

// Bool returns the amber boolean for b.
func BoolOf(b bool) Value {
	if b {
		return True
	}
	return False
}

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
