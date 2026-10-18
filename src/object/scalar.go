package object

import (
	"math"
	"math/big"
	"sort"
	"strconv"
)

// ScalarKind is what a cameo value of a scalar field type is.
type ScalarKind int

const (
	ScalarString ScalarKind = iota
	ScalarBytes
	ScalarBool
	ScalarFloat
	ScalarInteger
)

// ScalarType is a scalar field type: proto3's scalar types under their own
// names, `int` and `float` for Ruby's Integer and Float as int64 and double,
// `float32` for proto3's 32-bit float, whose own name `float` already means
// Ruby's Float, and `bignum` for an Integer of any size.
type ScalarType struct {
	Name string
	// Proto is the type's name in a .proto file.
	Proto string
	Kind  ScalarKind
	// Bits is the width of an integer or float type: 32 or 64. It is 0 for
	// bignum, which holds any Integer and travels as its decimal digits.
	Bits     int
	Unsigned bool
}

var scalarTypes = map[string]*ScalarType{}

func init() {
	add := func(name, proto string, kind ScalarKind, bits int, unsigned bool) {
		scalarTypes[name] = &ScalarType{Name: name, Proto: proto, Kind: kind, Bits: bits, Unsigned: unsigned}
	}
	add("string", "string", ScalarString, 0, false)
	add("bytes", "bytes", ScalarBytes, 0, false)
	add("bool", "bool", ScalarBool, 0, false)
	add("float", "double", ScalarFloat, 64, false)
	add("double", "double", ScalarFloat, 64, false)
	add("float32", "float", ScalarFloat, 32, false)
	add("int", "int64", ScalarInteger, 64, false)
	for _, name := range []string{"int32", "sint32", "sfixed32"} {
		add(name, name, ScalarInteger, 32, false)
	}
	for _, name := range []string{"int64", "sint64", "sfixed64"} {
		add(name, name, ScalarInteger, 64, false)
	}
	add("uint32", "uint32", ScalarInteger, 32, true)
	add("fixed32", "fixed32", ScalarInteger, 32, true)
	add("uint64", "uint64", ScalarInteger, 64, true)
	add("fixed64", "fixed64", ScalarInteger, 64, true)
	add("bignum", "string", ScalarInteger, 0, false)
	for name := range scalarTypes {
		scalarFields[name] = &FieldType{Kind: name, Scalar: scalarTypes[name]}
	}
}

var scalarFields = map[string]*FieldType{}

// Scalar returns the scalar field type of that name, or nil.
func Scalar(name string) *ScalarType { return scalarTypes[name] }

// ScalarField returns the field type of a scalar type's name, or nil. It is
// shared and must not be changed.
func ScalarField(name string) *FieldType { return scalarFields[name] }

// ScalarNames lists the scalar field types' names, sorted.
func ScalarNames() []string {
	names := make([]string, 0, len(scalarTypes))
	for name := range scalarTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MapKey reports whether protobuf takes the type as a map key: a string, a
// bool, or an integer of 32 or 64 bits.
func (t *ScalarType) MapKey() bool {
	switch t.Kind {
	case ScalarString, ScalarBool:
		return true
	case ScalarInteger:
		return t.Bits != 0
	}
	return false
}

// Fits reports whether an Integer is within an integer type's range.
func (t *ScalarType) Fits(v Value) bool {
	if t.Bits == 0 {
		return v.IsInteger()
	}
	switch v.kind {
	case KindInt:
		switch {
		case t.Unsigned:
			return v.n >= 0 && (t.Bits == 64 || v.n <= math.MaxUint32)
		case t.Bits == 32:
			return v.n >= math.MinInt32 && v.n <= math.MaxInt32
		}
		return true
	case KindBig:
		// Only an unsigned 64-bit type holds integers beyond int64.
		b := v.Big()
		return t.Unsigned && t.Bits == 64 && b.Sign() > 0 && b.IsUint64()
	}
	return false
}

// Range describes an integer type's range for an error message.
func (t *ScalarType) Range() string {
	switch {
	case t.Unsigned && t.Bits == 32:
		return "from 0 to " + strconv.FormatUint(math.MaxUint32, 10)
	case t.Unsigned:
		return "from 0 to " + strconv.FormatUint(math.MaxUint64, 10)
	case t.Bits == 32:
		return "from " + strconv.Itoa(math.MinInt32) + " to " + strconv.Itoa(math.MaxInt32)
	}
	return "from " + strconv.FormatInt(math.MinInt64, 10) + " to " + strconv.FormatInt(math.MaxInt64, 10)
}

// IntegerDigits is an Integer's decimal digits, the wire form of bignum.
func IntegerDigits(v Value) string {
	if v.kind == KindInt {
		return strconv.FormatInt(v.n, 10)
	}
	return v.Big().String()
}

// ParseInteger reads decimal digits into an Integer.
func ParseInteger(s string) (Value, bool) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Int(n), true
	}
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return Value{}, false
	}
	return BigInt(b), true
}

// Uint64Integer is the Integer of an unsigned 64-bit value.
func Uint64Integer(u uint64) Value {
	if u <= math.MaxInt64 {
		return Int(int64(u))
	}
	return BigInt(new(big.Int).SetUint64(u))
}

// IntegerUint64 is an Integer that Fits an unsigned type, as uint64.
func IntegerUint64(v Value) uint64 {
	if v.kind == KindInt {
		return uint64(v.n)
	}
	return v.Big().Uint64()
}
