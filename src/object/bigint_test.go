package object

import (
	"math"
	"math/big"
	"testing"
)

func TestBigIntHasOneForm(t *testing.T) {
	if v := BigInt(big.NewInt(42)); v.Kind() != KindInt || v.Int() != 42 {
		t.Fatalf("a big.Int that fits is a KindInt, got %s", v.Kind())
	}
	b, _ := new(big.Int).SetString("18446744073709551616", 10)
	v := BigInt(b)
	if v.Kind() != KindBig || v.Class() != IntegerClass {
		t.Fatalf("2**64 is a KindBig Integer, got %s of %s", v.Kind(), v.Class().Name)
	}
	if Key(v) != "i18446744073709551616" {
		t.Fatalf("Key(2**64) = %q", Key(v))
	}
	if !Equal(v, BigInt(new(big.Int).Set(b))) || Equal(v, Int(math.MaxInt64)) {
		t.Fatal("big integers compare by value")
	}
	if !Equal(v, Float(18446744073709551616.0)) {
		t.Fatal("2**64 == 2.0**64")
	}
}

func TestIntOverflow(t *testing.T) {
	for _, c := range []struct {
		name string
		f    func(x, y int64) (int64, bool)
		x, y int64
		ok   bool
	}{
		{"+", AddInt, math.MaxInt64, 1, false},
		{"+", AddInt, math.MaxInt64, -1, true},
		{"+", AddInt, math.MinInt64, -1, false},
		{"-", SubInt, math.MinInt64, 1, false},
		{"-", SubInt, 0, math.MinInt64, false},
		{"-", SubInt, -1, math.MinInt64, true},
		{"*", MulInt, 1 << 31, 1 << 31, true},
		{"*", MulInt, 1 << 32, 1 << 31, false},
		{"*", MulInt, 1 << 32, 1 << 32, false},
		{"*", MulInt, -1, math.MinInt64, false},
		{"*", MulInt, math.MinInt64, 1, true},
		{"*", MulInt, 0, math.MinInt64, true},
	} {
		if _, ok := c.f(c.x, c.y); ok != c.ok {
			t.Errorf("%d %s %d: ok = %v, want %v", c.x, c.name, c.y, ok, c.ok)
		}
	}
}

func TestCompareNumbersIsExact(t *testing.T) {
	big1, _ := new(big.Int).SetString("9007199254740993", 10) // 2**53 + 1
	// 2**53 + 1 rounds to the float 2**53, but is larger than it.
	if r, ok := CompareNumbers(BigInt(big1), Float(9007199254740992)); !ok || r != 1 {
		t.Fatalf("2**53+1 <=> 2.0**53 = %d, %v", r, ok)
	}
	if _, ok := CompareNumbers(Int(1), Float(math.NaN())); ok {
		t.Fatal("NaN does not compare")
	}
}
