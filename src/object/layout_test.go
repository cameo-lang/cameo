package object

import (
	"math"
	"math/big"
	"testing"
	"unsafe"

	"github.com/cameo-lang/cameo/src/assert"
)

// A Value is two words, and the zero Value is no value.
func TestAValueIsTwoWords(t *testing.T) {
	assert.Equals(t, int(unsafe.Sizeof(Value{})), 16)
	assert.True(t, Value{}.IsZero())
	assert.Equals(t, Value{}.Kind(), KindNone)
	assert.False(t, Nil.IsZero())
}

// A value of a kind that lives in the payload has all 64 bits of it, and
// reads back as its kind.
func TestPayloadKindsKeepSixtyFourBits(t *testing.T) {
	for _, i := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64, 1 << 56, -(1 << 56), 1<<63 - 1} {
		v := Int(i)
		assert.Equals(t, v.Kind(), KindInt)
		got, ok := v.AsInt()
		assert.True(t, ok)
		assert.Equals(t, got, i)
	}
	for _, f := range []float64{0, -0.0, 1.5, -1.5, math.MaxFloat64, math.SmallestNonzeroFloat64, math.Inf(1), math.Inf(-1)} {
		v := Float(f)
		assert.Equals(t, v.Kind(), KindFloat)
		assert.Equals(t, math.Float64bits(v.Float()), math.Float64bits(f))
	}
	assert.True(t, math.IsNaN(Float(math.NaN()).Float()))
	assert.Equals(t, True.Kind(), KindBool)
	assert.True(t, True.Truthy())
	assert.False(t, False.Truthy())
	assert.Equals(t, Nil.Kind(), KindNil)
}

// A string and a symbol read back as their kind and their text, the one
// of no bytes too, in every encoding.
func TestStringsKeepTheirKindAndText(t *testing.T) {
	long := string(make([]byte, 1<<20))
	for _, k := range []Kind{KindString, KindASCII} {
		for _, s := range []string{"", "a", "héllo", long} {
			v := StringOf(k, s)
			assert.Equals(t, v.Kind(), k)
			assert.True(t, v.Str() == s)
		}
	}
	assert.Equals(t, Symbol("").Kind(), KindSymbol)
	assert.Equals(t, Symbol("").Str(), "")
	assert.Equals(t, Symbol("name").Str(), "name")
	assert.Equals(t, Bytes(nil).Kind(), KindBytes)
	assert.Equals(t, len(Bytes(nil).Bytes()), 0)
	assert.Equals(t, string(Bytes([]byte{0xff, 0}).Bytes()), "\xff\x00")
	assert.False(t, Same(String(""), Symbol("")))
	assert.False(t, Same(String(""), Nil))
	assert.False(t, Same(Int(0), False))
	assert.False(t, Same(Int(0), Float(0)))
}

// A value that is an object reads back as its kind and its object, and
// an opaque value of no object keeps its kind.
func TestObjectsKeepTheirKind(t *testing.T) {
	l := NewList(Int(1)).List()
	v := ListValue(l)
	assert.Equals(t, v.Kind(), KindList)
	assert.True(t, v.List() == l)
	assert.True(t, Same(v, ListValue(l)))
	b := BigInt(new(big.Int).Lsh(big.NewInt(1), 100))
	assert.Equals(t, b.Kind(), KindBig)
	assert.Equals(t, ClassValue(ObjectClass).Kind(), KindClass)
	none := Opaque(KindProc, nil)
	assert.Equals(t, none.Kind(), KindProc)
	assert.False(t, none.IsZero())
	assert.True(t, none.Opaque() == nil)
}
