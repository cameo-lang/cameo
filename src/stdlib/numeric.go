package stdlib

import (
	"math"
	"strings"

	"github.com/radiospiel/cameo/src/object"
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
	arith("%", floatMod)
	arith("**", math.Pow)
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
		q := math.Floor(a / b)
		return object.NewList(object.Float(q), object.Float(a-q*b)), nil
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
	rounding("round", roundHalfAway)
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
	c.Consts["MIN"] = object.Float(2.2250738585072014e-308)
	_ = strings.TrimSpace
}

func roundHalfAway(x float64) float64 {
	return math.Round(x)
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
