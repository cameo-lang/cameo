package object

import (
	"math"
	"math/big"
	"unsafe"
)

// An Integer is a KindInt while it fits in 64 bits and a KindBig, which
// points at its digits, once it does not: arithmetic that overflows moves
// to a big integer and a result that fits moves back, so one number has one
// form, and Key and Equal can compare forms.

// BigInt makes an integer value of b, a KindInt when it fits; b must not be
// changed afterwards.
func BigInt(b *big.Int) Value {
	if b.IsInt64() {
		return Int(b.Int64())
	}
	return Value{kind: KindBig, p: unsafe.Pointer(b)}
}

// Big returns the digits of a big integer; they must not be changed.
func (v Value) Big() *big.Int {
	v.must(KindBig)
	return (*big.Int)(v.p)
}

// AsBigInt reads either form of an integer as a big.Int, which for a
// KindBig is its own and must not be changed.
func (v Value) AsBigInt() (*big.Int, bool) {
	switch v.kind {
	case KindInt:
		return big.NewInt(v.n), true
	case KindBig:
		return (*big.Int)(v.p), true
	}
	return nil, false
}

// IsInteger reports an Integer in either form.
func (v Value) IsInteger() bool { return v.kind == KindInt || v.kind == KindBig }

// BigToFloat is the float nearest b, ±Inf beyond the float range.
func BigToFloat(b *big.Int) float64 {
	f, _ := new(big.Float).SetInt(b).Float64()
	return f
}

// FloatToInteger is the integer part of a finite float, in either form.
func FloatToInteger(f float64) Value {
	if f >= -(1<<63) && f < 1<<63 {
		return Int(int64(f))
	}
	b, _ := big.NewFloat(math.Trunc(f)).Int(nil)
	return BigInt(b)
}

// compareNumbers orders two numbers, either integer form or a float,
// exactly; ok is false when either is NaN.
func compareNumbers(a, b Value) (r int, ok bool) {
	if a.kind == KindInt && b.kind == KindInt {
		switch {
		case a.n < b.n:
			return -1, true
		case a.n > b.n:
			return 1, true
		}
		return 0, true
	}
	x, y := exactFloat(a), exactFloat(b)
	if x == nil || y == nil {
		return 0, false
	}
	return x.Cmp(y), true
}

func exactFloat(v Value) *big.Float {
	switch v.kind {
	case KindInt:
		return new(big.Float).SetInt64(v.n)
	case KindBig:
		return new(big.Float).SetInt(v.Big())
	case KindFloat:
		f := v.Float()
		if math.IsNaN(f) {
			return nil
		}
		return big.NewFloat(f)
	}
	return nil
}

// CompareNumbers orders two numbers exactly, for Integer and Float's <=>:
// a big integer against a float compares their values, not a rounding.
// ok is false when either is not a number or is NaN.
func CompareNumbers(a, b Value) (int, bool) {
	if !a.isNumber() || !b.isNumber() {
		return 0, false
	}
	return compareNumbers(a, b)
}

func (v Value) isNumber() bool { return v.kind == KindInt || v.kind == KindBig || v.kind == KindFloat }

// AddInt, SubInt and MulInt are 64-bit arithmetic that reports overflow,
// where an Integer moves to its big form.
func AddInt(x, y int64) (int64, bool) {
	r := x + y
	return r, (r^x)&(r^y) >= 0
}

func SubInt(x, y int64) (int64, bool) {
	r := x - y
	return r, (x^y)&(x^r) >= 0
}

func MulInt(x, y int64) (int64, bool) {
	if x == 0 || y == 0 {
		return 0, true
	}
	r := x * y
	if r/y != x || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64) {
		return 0, false
	}
	return r, true
}
