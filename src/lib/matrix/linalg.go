package matrix

import (
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
)

// The inverse runs Ruby's Gauss-Jordan elimination, inverse_from, over
// one of two number fields: exact rationals for a matrix of Integers,
// where Ruby's quo makes Rationals, and float64 for one with a Float,
// where Ruby's does too. Each result goes back to a script as a Float,
// since cameo has no Rational.

// field is the arithmetic of a number type.
type field[T any] struct {
	add, sub, mul, quo func(a, b T) T
	isZero             func(T) bool
	abs                func(T) T
	less               func(a, b T) bool
	zero, one          T
}

var ratField = field[*big.Rat]{
	add:    func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) },
	sub:    func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) },
	mul:    func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) },
	quo:    func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) },
	isZero: func(a *big.Rat) bool { return a.Sign() == 0 },
	abs:    func(a *big.Rat) *big.Rat { return new(big.Rat).Abs(a) },
	less:   func(a, b *big.Rat) bool { return a.Cmp(b) < 0 },
	zero:   new(big.Rat),
	one:    big.NewRat(1, 1),
}

var floatField = field[float64]{
	add:    func(a, b float64) float64 { return a + b },
	sub:    func(a, b float64) float64 { return a - b },
	mul:    func(a, b float64) float64 { return a * b },
	quo:    func(a, b float64) float64 { return a / b },
	isZero: func(a float64) bool { return a == 0 },
	abs:    math.Abs,
	less:   func(a, b float64) bool { return a < b },
	zero:   0,
	one:    1,
}

// errNotRegular says the matrix has no inverse.
type errNotRegular struct{}

func (errNotRegular) Error() string { return "Not Regular Matrix" }

// inverseOf is Matrix#inverse_from: a is changed, the identity it starts
// from becomes the inverse.
func inverseOf[T any](a [][]T, f field[T]) ([][]T, error) {
	n := len(a)
	out := make([][]T, n)
	for i := range out {
		out[i] = make([]T, n)
		for j := range out[i] {
			out[i][j] = f.zero
		}
		out[i][i] = f.one
	}
	for k := 0; k < n; k++ {
		i := k
		akk := f.abs(a[k][k])
		for j := k + 1; j < n; j++ {
			if v := f.abs(a[j][k]); f.less(akk, v) {
				i, akk = j, v
			}
		}
		if f.isZero(akk) {
			return nil, errNotRegular{}
		}
		if i != k {
			a[i], a[k] = a[k], a[i]
			out[i], out[k] = out[k], out[i]
		}
		pivot := a[k][k]
		for ii := 0; ii < n; ii++ {
			if ii == k {
				continue
			}
			q := f.quo(a[ii][k], pivot)
			a[ii][k] = f.zero
			for j := k + 1; j < n; j++ {
				a[ii][j] = f.sub(a[ii][j], f.mul(a[k][j], q))
			}
			for j := 0; j < n; j++ {
				out[ii][j] = f.sub(out[ii][j], f.mul(out[k][j], q))
			}
		}
		for j := k + 1; j < n; j++ {
			a[k][j] = f.quo(a[k][j], pivot)
		}
		for j := 0; j < n; j++ {
			out[k][j] = f.quo(out[k][j], pivot)
		}
	}
	return out, nil
}

// exactRows reads a matrix of Integers as rationals; ok is false when an
// element is not an Integer.
func exactRows(rows [][]object.Value) ([][]*big.Rat, bool) {
	out := make([][]*big.Rat, len(rows))
	for i, row := range rows {
		out[i] = make([]*big.Rat, len(row))
		for j, v := range row {
			switch v.Kind() {
			case object.KindInt:
				out[i][j] = big.NewRat(v.Int(), 1)
			case object.KindBig:
				out[i][j] = new(big.Rat).SetInt(v.Big())
			default:
				return nil, false
			}
		}
	}
	return out, true
}

// floatRows reads a matrix of numbers as float64s; ok is false when an
// element is not a number.
func floatRows(rows [][]object.Value) ([][]float64, bool) {
	out := make([][]float64, len(rows))
	for i, row := range rows {
		out[i] = make([]float64, len(row))
		for j, v := range row {
			f, ok := v.AsNumber()
			if !ok {
				return nil, false
			}
			out[i][j] = f
		}
	}
	return out, true
}

// inverseRows is the inverse of a square matrix of numbers as Floats.
func inverseRows(rows [][]object.Value) ([][]object.Value, error) {
	n := len(rows)
	var out [][]object.Value
	if exact, ok := exactRows(rows); ok {
		inv, err := inverseOf(exact, ratField)
		if err != nil {
			return nil, err
		}
		out = make([][]object.Value, n)
		for i := range inv {
			out[i] = make([]object.Value, n)
			for j, r := range inv[i] {
				f, _ := r.Float64()
				out[i][j] = object.Float(f)
			}
		}
		return out, nil
	}
	floats, ok := floatRows(rows)
	if !ok {
		return nil, errNotNumber{}
	}
	inv, err := inverseOf(floats, floatField)
	if err != nil {
		return nil, err
	}
	out = make([][]object.Value, n)
	for i := range inv {
		out[i] = make([]object.Value, n)
		for j, f := range inv[i] {
			out[i][j] = object.Float(f)
		}
	}
	return out, nil
}

// errNotNumber says an element is not a number, so the arithmetic has
// nothing to work on.
type errNotNumber struct{}

func (errNotNumber) Error() string { return "the elements must be numbers" }
