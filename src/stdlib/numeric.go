package stdlib

import (
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
)

func installNumeric() {
	installInteger()
	installFloat()
}

// numArgs reads two numbers as floats, and reports whether both are
// integers.
func numArgs(ctx object.Context, self, other object.Value, op string) (a, b float64, ints bool, err error) {
	a, _ = self.AsNumber()
	b, ok := other.AsNumber()
	if !ok {
		return 0, 0, false, ctx.Raise(object.TypeError, "%s can't be coerced into %s", kindName(other), self.Class().Name)
	}
	return a, b, self.IsInteger() && other.IsInteger(), nil
}

func installFloat() {
	c := object.FloatClass
	def(c, "step", -1, numericStep)
	arith := func(name string, f func(a, b float64) float64) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, b, _, err := numArgs(ctx, self, args[0], name)
			if err != nil {
				return object.Value{}, err
			}
			return object.Float(f(a, b)), nil
		})
	}
	arith("+", func(a, b float64) float64 { return a + b })
	arith("-", func(a, b float64) float64 { return a - b })
	arith("*", func(a, b float64) float64 { return a * b })
	arith("/", func(a, b float64) float64 { return a / b })
	arith("**", floatPow)
	def(c, "%", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "%")
		if err != nil {
			return object.Value{}, err
		}
		if b == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return object.Float(floatMod(a, b)), nil
	})
	def(c, "modulo", 1, c.Methods["%"].Native)
	def(c, "fdiv", 1, c.Methods["/"].Native)
	def(c, "pow", 1, c.Methods["**"].Native)
	def(c, "div", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "div")
		if err != nil {
			return object.Value{}, err
		}
		if b == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return floatToInteger(ctx, math.Floor(a/b))
	})
	def(c, "divmod", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "divmod")
		if err != nil {
			return object.Value{}, err
		}
		if b == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		q, m := floatDivmod(a, b)
		qi, err := floatToInteger(ctx, q)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(qi, object.Float(m)), nil
	})
	def(c, "-@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(-self.Float()), nil
	})
	def(c, "+@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "<=>", 1, compareNumber)
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
	def(c, "abs", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(math.Abs(float64(self.Float()))), nil
	})
	def(c, "magnitude", 0, c.Methods["abs"].Native)
	pred := func(name string, f func(float64) bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Bool(f(float64(self.Float()))), nil
		})
	}
	pred("zero?", func(f float64) bool { return f == 0 })
	pred("positive?", func(f float64) bool { return f > 0 })
	pred("negative?", func(f float64) bool { return f < 0 })
	pred("nan?", math.IsNaN)
	pred("finite?", func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) })
	pred("integer?", func(float64) bool { return false })
	def(c, "infinite?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f := float64(self.Float())
		switch {
		case math.IsInf(f, 1):
			return object.Int(1), nil
		case math.IsInf(f, -1):
			return object.Int(-1), nil
		}
		return object.Nil, nil
	})
	def(c, "nonzero?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.Float() == 0 {
			return object.Nil, nil
		}
		return self, nil
	})
	def(c, "round", -1, floatRound)
	def(c, "coerce", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return coerceFloats(ctx, self, args[0])
	})
	rounding := func(name string, f func(float64) float64) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			x := float64(self.Float())
			if len(args) == 0 {
				if math.IsNaN(x) || math.IsInf(x, 0) {
					return object.Value{}, ctx.Raise(object.RangeError, "%s", object.FormatFloat(x))
				}
				return object.FloatToInteger(f(x)), nil
			}
			digits, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			p := math.Pow(10, float64(digits))
			r := f(x*p) / p
			if digits <= 0 {
				return floatToInteger(ctx, r)
			}
			return object.Float(r), nil
		})
	}
	rounding("floor", math.Floor)
	rounding("ceil", math.Ceil)
	rounding("truncate", math.Trunc)
	def(c, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		x := float64(self.Float())
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return object.Value{}, ctx.Raise(object.RangeError, "%s", object.FormatFloat(x))
		}
		return object.FloatToInteger(x), nil
	})
	def(c, "to_int", 0, c.Methods["to_i"].Native)
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(object.FormatFloat(float64(self.Float()))), nil
	})
	def(c, "inspect", 0, c.Methods["to_s"].Native)
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	object.ObjectClass.Consts["Float"] = object.ClassValue(c)
	c.Consts["INFINITY"] = object.Float(math.Inf(1))
	c.Consts["NAN"] = object.Float(math.NaN())
	c.Consts["EPSILON"] = object.Float(2.220446049250313e-16)
	c.Consts["MAX"] = object.Float(math.MaxFloat64)
	def(c, "next_float", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(math.Nextafter(self.Float(), math.Inf(1))), nil
	})
	def(c, "prev_float", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(math.Nextafter(self.Float(), math.Inf(-1))), nil
	})
	// arg is the angle of the float on the complex plane: pi for a negative
	// float, -0.0 included, and NaN for NaN.
	arg := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch x := self.Float(); {
		case math.IsNaN(x):
			return self, nil
		case math.Signbit(x):
			return object.Float(math.Pi), nil
		}
		return object.Int(0), nil
	}
	def(c, "arg", 0, arg)
	def(c, "angle", 0, arg)
	def(c, "phase", 0, arg)
	def(c, "remainder", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		x, y, _, err := numArgs(ctx, self, args[0], "remainder")
		if err != nil {
			return object.Value{}, err
		}
		if y == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return object.Float(math.Mod(x, y)), nil
	})
	c.Consts["MIN"] = object.Float(2.2250738585072014e-308)
	c.Consts["DIG"] = object.Int(15)
	c.Consts["MANT_DIG"] = object.Int(53)
	c.Consts["MAX_10_EXP"] = object.Int(308)
	c.Consts["MIN_10_EXP"] = object.Int(-307)
	c.Consts["MAX_EXP"] = object.Int(1024)
	c.Consts["MIN_EXP"] = object.Int(-1021)
	c.Consts["RADIX"] = object.Int(2)
}

// floatPow is x**y. Go's math.Pow multiplies in floats for a whole y,
// and is off by a few units in the last place for 10.0**308; C's pow,
// which Ruby calls, is correctly rounded there. For a whole y of a
// moderate size floatPow computes the power exactly and rounds once.
func floatPow(x, y float64) float64 {
	const maxExact = 1100
	if x == 0 || math.IsInf(x, 0) || math.IsNaN(x) || y != math.Trunc(y) || math.Abs(y) < 2 || math.Abs(y) > maxExact {
		return math.Pow(x, y)
	}
	n := int(math.Abs(y))
	prec := uint(53*n + 64)
	base := new(big.Float).SetPrec(prec).SetFloat64(x)
	r := new(big.Float).SetPrec(prec).SetInt64(1)
	for e := n; e > 0; e >>= 1 {
		if e&1 == 1 {
			r.Mul(r, base)
		}
		base.Mul(base, base)
	}
	if y < 0 {
		r.Quo(new(big.Float).SetPrec(prec).SetInt64(1), r)
	}
	f, _ := r.Float64()
	return f
}

// coerceFloats is Numeric#coerce for an other that is not of the
// receiver's class: both as floats, other first.
func coerceFloats(ctx object.Context, self, other object.Value) (object.Value, error) {
	o, err := toFloat(ctx, other)
	if err != nil {
		return object.Value{}, err
	}
	s, _ := self.AsNumber()
	return object.NewList(o, object.Float(s)), nil
}

// floatDivmod is Ruby's flodivmod: the floored quotient, a whole float,
// and the modulus with the sign of y. y is not zero.
func floatDivmod(x, y float64) (div, mod float64) {
	if math.IsNaN(y) {
		return y, y
	}
	if x == 0 || math.IsInf(y, 0) && !math.IsInf(x, 0) {
		mod = x
	} else {
		mod = math.Mod(x, y)
	}
	if math.IsInf(x, 0) && !math.IsInf(y, 0) {
		div = x
	} else {
		div = math.Round((x - mod) / y)
	}
	if y*mod < 0 {
		mod += y
		div--
	}
	return div, mod
}

// roundingMode reads the half: keyword of round: "up" (the default, also
// for nil), "even" or "down".
func roundingMode(ctx object.Context, kw *object.Map) (string, error) {
	if kw == nil || kw.Len() == 0 {
		return "up", nil
	}
	for _, k := range kw.Keys() {
		if k.Kind() != object.KindSymbol || k.Str() != "half" {
			s, err := ctx.Inspect(k)
			if err != nil {
				return "", err
			}
			return "", ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
		}
	}
	v, _ := kw.Get(object.Symbol("half"))
	if v.IsNil() {
		return "up", nil
	}
	var mode string
	switch v.Kind() {
	case object.KindSymbol, object.KindString, object.KindASCII:
		mode = v.Str()
	default:
		s, err := ctx.Inspect(v)
		if err != nil {
			return "", err
		}
		return "", ctx.Raise(object.ArgumentError, "invalid rounding mode: %s", s)
	}
	switch mode {
	case "up", "even", "down":
		return mode, nil
	}
	return "", ctx.Raise(object.ArgumentError, "invalid rounding mode: %s", mode)
}

// floatRound is Float#round(ndigits = 0, half: :up), after Ruby's
// flo_round: a negative ndigits rounds the integer part, exactly; up to 14
// digits round in floats, checking each candidate against x so that the
// error in x * 10**ndigits does not move the result; more round exactly.
func floatRound(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, argError(ctx, len(args), "0..1")
	}
	var ndigits int64
	if len(args) == 1 {
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n > math.MaxInt32 || n < math.MinInt32 {
			return object.Value{}, ctx.Raise(object.RangeError, "integer %d too big to convert to 'int'", n)
		}
		ndigits = n
	}
	mode, err := roundingMode(ctx, kw)
	if err != nil {
		return object.Value{}, err
	}
	x := self.Float()
	if x == 0 {
		if ndigits > 0 {
			return self, nil
		}
		return object.Int(0), nil
	}
	if ndigits < 0 {
		i, err := floatToInteger(ctx, x)
		if err != nil {
			return object.Value{}, err
		}
		return roundInteger(i, ndigits, mode), nil
	}
	if ndigits == 0 {
		return floatToInteger(ctx, roundFloat(x, 1, mode))
	}
	_, binexp := math.Frexp(x)
	if math.IsInf(x, 0) || math.IsNaN(x) {
		binexp = 0
	}
	if floatRoundOverflow(ndigits, binexp) {
		return self, nil
	}
	if floatRoundUnderflow(ndigits, binexp) {
		return object.Float(0), nil
	}
	if ndigits > 14 {
		return object.Float(roundFloatExactly(x, ndigits, mode)), nil
	}
	s := math.Pow10(int(ndigits))
	return object.Float(roundFloat(x, s, mode) / s), nil
}

// floatRoundOverflow reports that x * 10**ndigits is a whole number
// already, so that rounding returns x: a float has at most 17 significant
// digits, and x has about binexp/3.32 of them before the point.
func floatRoundOverflow(ndigits int64, binexp int) bool {
	const floatDig = 17
	e := binexp/3 - 1
	if binexp > 0 {
		e = binexp / 4
	}
	return ndigits >= int64(floatDig-e)
}

// floatRoundUnderflow reports that x rounds to zero at ndigits.
func floatRoundUnderflow(ndigits int64, binexp int) bool {
	e := binexp / 4
	if binexp > 0 {
		e = binexp/3 + 1
	}
	return ndigits < int64(-e)
}

// roundFloat rounds x * s to a whole number by mode, after Ruby's
// round_half_up and round_half_down: x * s carries a rounding error, so
// each candidate is checked against x itself. A tie for even is where up
// and down differ.
func roundFloat(x, s float64, mode string) float64 {
	up := func() float64 {
		f := math.Round(x * s)
		if s == 1 {
			return f
		}
		if x > 0 && (f+0.5)/s <= x {
			f++
		} else if x < 0 && (f-0.5)/s >= x {
			f--
		}
		return f
	}
	down := func() float64 {
		f := math.Round(x * s)
		if x > 0 && (f-0.5)/s >= x {
			f--
		} else if x < 0 && (f+0.5)/s <= x {
			f++
		}
		return f
	}
	switch mode {
	case "down":
		return down()
	case "even":
		u, d := up(), down()
		if u == d || math.Mod(d, 2) == 0 {
			return d
		}
		return u
	}
	return up()
}

// roundFloatExactly rounds x to ndigits decimal digits in exact
// arithmetic, for ndigits where 10**ndigits is not exact as a float.
func roundFloatExactly(x float64, ndigits int64, mode string) float64 {
	r := new(big.Rat).SetFloat64(x)
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(ndigits), nil)
	r.Mul(r, new(big.Rat).SetInt(p))
	q, m := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	twice := new(big.Int).Abs(m)
	twice.Lsh(twice, 1)
	c := twice.Cmp(r.Denom())
	if c > 0 || c == 0 && (mode == "up" || mode == "even" && q.Bit(0) == 1) {
		if r.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	f, _ := new(big.Rat).SetFrac(q, p).Float64()
	return f
}

// numericStep is Numeric#step(limit = nil, step = 1), or with the to: and
// by: keywords: integers when all three are integers, otherwise floats as
// Ruby's ruby_float_step makes them, each one beg + i * step so that no
// error accumulates, and never past the limit. Without a block it returns
// the numbers.
func numericStep(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 2 {
		return object.Value{}, argError(ctx, len(args), "0..2")
	}
	limit, by := object.Nil, object.Int(1)
	if len(args) > 0 {
		limit = args[0]
	}
	if len(args) > 1 {
		by = args[1]
	}
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("to")); ok {
			limit = v
		}
		if v, ok := kw.Get(object.Symbol("by")); ok {
			by = v
		}
	}
	var out []object.Value
	emit := func(v object.Value) error {
		if block.IsZero() {
			out = append(out, v)
			return nil
		}
		_, err := ctx.Yield(block, v)
		return err
	}
	done := func() (object.Value, error) {
		if block.IsZero() {
			return object.NewList(out...), nil
		}
		return self, nil
	}
	for _, v := range []object.Value{by, limit} {
		if _, ok := v.AsNumber(); !ok && !v.IsNil() {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(v))
		}
	}
	if f, _ := by.AsNumber(); f == 0 {
		return object.Value{}, ctx.Raise(object.ArgumentError, "step can't be 0")
	}
	if limit.IsNil() && block.IsZero() {
		return object.Value{}, ctx.Raise(object.RangeError, "cannot convert endless range to a list")
	}
	from, fromInt := self.AsInt()
	to, toInt := limit.AsInt()
	step, stepInt := by.AsInt()
	// In 64 bits while no step can pass the end of them.
	if fromInt && stepInt && toInt && (step > 0 && to <= math.MaxInt64-step || step < 0 && to >= math.MinInt64-step) {
		for i := from; step > 0 && i <= to || step < 0 && i >= to; i += step {
			if err := emit(object.Int(i)); err != nil {
				return object.Value{}, err
			}
		}
		return done()
	}
	if self.IsInteger() && by.IsInteger() && (limit.IsInteger() || limit.IsNil()) {
		up := bigOf(by).Sign() > 0
		for i := self; ; i = addIntegers(i, by) {
			if !limit.IsNil() {
				if c, _ := object.CompareNumbers(i, limit); up && c > 0 || !up && c < 0 {
					return done()
				}
			}
			if err := emit(i); err != nil {
				return object.Value{}, err
			}
		}
	}
	beg, _ := self.AsNumber()
	unit, _ := by.AsNumber()
	end := math.Inf(1)
	if unit < 0 {
		end = math.Inf(-1)
	}
	if !limit.IsNil() {
		end, _ = limit.AsNumber()
	}
	n := floatStepSize(beg, end, unit)
	if math.IsInf(unit, 0) {
		// i * unit is NaN for i = 0, so Ruby yields beg alone.
		if n > 0 {
			if err := emit(object.Float(beg)); err != nil {
				return object.Value{}, err
			}
		}
		return done()
	}
	for i := float64(0); i < n; i++ {
		d := i*unit + beg
		if unit >= 0 && end < d || unit < 0 && d < end {
			d = end
		}
		if err := emit(object.Float(d)); err != nil {
			return object.Value{}, err
		}
	}
	return done()
}

// floatStepSize is how many floats a step from beg to end by unit makes,
// allowing for the rounding error in (end - beg) / unit, after Ruby's
// ruby_float_step_size.
func floatStepSize(beg, end, unit float64) float64 {
	if math.IsInf(unit, 0) {
		if unit > 0 && beg <= end || unit < 0 && beg >= end {
			return 1
		}
		return 0
	}
	n := (end - beg) / unit
	err := (math.Abs(beg) + math.Abs(end) + math.Abs(end-beg)) / math.Abs(unit) * epsilon
	if err > 0.5 {
		err = 0.5
	}
	if n < 0 {
		return 0
	}
	return math.Floor(n+err) + 1
}

// epsilon is the difference between 1 and the next float, C's DBL_EPSILON.
const epsilon = 2.220446049250313e-16
