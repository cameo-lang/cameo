package bigdecimal

import (
	"math/big"
)

// BigMath's functions, and BigDecimal ** with a fractional exponent, run
// on big.Float with enough bits for the digits asked for, and the result
// is rounded to those digits.

func bitsFor(digits int64) uint { return uint(float64(digits+8)*3.33) + 64 }

// toFloat converts a finite decimal to a big.Float of prec bits.
func (d dec) toBigFloat(prec uint) *big.Float {
	f, _, _ := big.ParseFloat(d.engineering("", 0), 10, prec, big.ToNearestEven)
	return f
}

// fromBigFloat rounds a big.Float to n significant digits, half up.
func fromBigFloat(f *big.Float, n int64) dec {
	if f.Sign() == 0 {
		return zero(false)
	}
	s := f.Text('e', int(n)+6)
	d, _ := parse(s)
	return roundSig(d, n, roundHalfUp)
}

func newF(prec uint) *big.Float { return new(big.Float).SetPrec(prec) }

func floatInt(prec uint, n int64) *big.Float { return newF(prec).SetInt64(n) }

// atanSmall is the Taylor series of atan for |x| < 1, run until the term
// is below the precision.
func atanSmall(x *big.Float, prec uint) *big.Float {
	sum := newF(prec).Set(x)
	x2 := newF(prec).Mul(x, x)
	term := newF(prec).Set(x)
	eps := newF(prec).SetMantExp(floatInt(prec, 1), -int(prec))
	for k := int64(1); ; k++ {
		term.Mul(term, x2)
		t := newF(prec).Quo(term, floatInt(prec, 2*k+1))
		if k%2 == 1 {
			sum.Sub(sum, t)
		} else {
			sum.Add(sum, t)
		}
		if newF(prec).Abs(t).Cmp(eps) < 0 {
			return sum
		}
	}
}

// piF is pi by Machin's formula.
func piF(prec uint) *big.Float {
	inv := func(n int64) *big.Float { return newF(prec).Quo(floatInt(prec, 1), floatInt(prec, n)) }
	a := atanSmall(inv(5), prec)
	b := atanSmall(inv(239), prec)
	a.Mul(a, floatInt(prec, 16))
	b.Mul(b, floatInt(prec, 4))
	return a.Sub(a, b)
}

// expF is e^x: the argument halved until small, the series summed, and
// the result squared back.
func expF(x *big.Float, prec uint) *big.Float {
	work := prec + 32
	y := newF(work).Set(x)
	half := newF(work).SetFloat64(0.5)
	halvings := 0
	for newF(work).Abs(y).Cmp(half) > 0 {
		y.Mul(y, half)
		halvings++
	}
	sum := floatInt(work, 1)
	term := floatInt(work, 1)
	eps := newF(work).SetMantExp(floatInt(work, 1), -int(work))
	for k := int64(1); ; k++ {
		term.Mul(term, y)
		term.Quo(term, floatInt(work, k))
		sum.Add(sum, term)
		if newF(work).Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	for ; halvings > 0; halvings-- {
		sum.Mul(sum, sum)
	}
	return newF(prec).Set(sum)
}

// logF is the natural logarithm of x > 0: x = m × 2^k with m in [0.5, 1),
// ln m by the atanh series, ln 2 the same way.
func logF(x *big.Float, prec uint) *big.Float {
	work := prec + 32
	m := newF(work)
	k := x.MantExp(m) // x = m × 2^k, m in [0.5, 1)
	lnm := lnNear1(m, work)
	ln2 := lnNear1(floatInt(work, 2), work)
	ln2.Mul(ln2, floatInt(work, int64(k)))
	return newF(prec).Add(lnm, ln2)
}

// lnNear1 is ln(m) = 2 atanh((m-1)/(m+1)), which converges for any m > 0
// and fast near 1.
func lnNear1(m *big.Float, prec uint) *big.Float {
	one := floatInt(prec, 1)
	z := newF(prec).Quo(newF(prec).Sub(m, one), newF(prec).Add(m, one))
	z2 := newF(prec).Mul(z, z)
	sum := newF(prec).Set(z)
	term := newF(prec).Set(z)
	eps := newF(prec).SetMantExp(floatInt(prec, 1), -int(prec))
	for k := int64(1); ; k++ {
		term.Mul(term, z2)
		t := newF(prec).Quo(term, floatInt(prec, 2*k+1))
		sum.Add(sum, t)
		if newF(prec).Abs(t).Cmp(eps) < 0 {
			break
		}
	}
	return sum.Mul(sum, floatInt(prec, 2))
}

// sinCosF reduces x modulo 2 pi and sums the series.
func sinCosF(x *big.Float, prec uint, cosine bool) *big.Float {
	work := prec + 32
	twoPi := piF(work)
	twoPi.Mul(twoPi, floatInt(work, 2))
	y := newF(work).Set(x)
	// y = y - round(y / 2pi) × 2pi
	q := newF(work).Quo(y, twoPi)
	qi, _ := q.Int(nil)
	y.Sub(y, newF(work).Mul(twoPi, newF(work).SetInt(qi)))
	y2 := newF(work).Mul(y, y)
	var sum, term *big.Float
	if cosine {
		sum = floatInt(work, 1)
		term = floatInt(work, 1)
	} else {
		sum = newF(work).Set(y)
		term = newF(work).Set(y)
	}
	eps := newF(work).SetMantExp(floatInt(work, 1), -int(work))
	for k := int64(1); ; k++ {
		var a, b int64
		if cosine {
			a, b = 2*k-1, 2*k
		} else {
			a, b = 2*k, 2*k+1
		}
		term.Mul(term, y2)
		term.Quo(term, floatInt(work, a*b))
		term.Neg(term)
		sum.Add(sum, term)
		if newF(work).Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	return newF(prec).Set(sum)
}

// atanF halves the argument, atan(x) = 2 atan(x / (1 + sqrt(1 + x^2))),
// until the series converges fast.
func atanF(x *big.Float, prec uint) *big.Float {
	work := prec + 32
	y := newF(work).Set(x)
	limit := newF(work).SetFloat64(0.2)
	doublings := int64(1)
	one := floatInt(work, 1)
	for newF(work).Abs(y).Cmp(limit) > 0 {
		s := newF(work).Mul(y, y)
		s.Add(s, one)
		s.Sqrt(s)
		s.Add(s, one)
		y.Quo(y, s)
		doublings *= 2
	}
	r := atanSmall(y, work)
	r.Mul(r, floatInt(work, doublings))
	return newF(prec).Set(r)
}

// powerFloat is x ** y for a fractional y: exp(y × ln x) to the digits
// bigdecimal gives such a power.
func powerFloat(x, y dec, digits int64) dec {
	prec := bitsFor(digits)
	xf := x.toBigFloat(prec + 32)
	yf := y.toBigFloat(prec + 32)
	l := logF(xf, prec+32)
	l.Mul(l, yf)
	return fromBigFloat(expF(l, prec+32), digits)
}
