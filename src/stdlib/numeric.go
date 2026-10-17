package stdlib

import (
	"math"
	"strconv"
	"strings"

	"github.com/radiospiel/amber/src/object"
)

func installNumeric() {
	installInteger()
	installFloat()
}

// numArgs reads two numbers as floats, and reports whether both are
// integers.
func numArgs(ctx object.Context, self, other object.Value, op string) (a, b float64, ints bool, err error) {
	ai, aIsInt := self.(object.Int)
	bi, bIsInt := other.(object.Int)
	if aIsInt && bIsInt {
		return float64(ai), float64(bi), true, nil
	}
	switch v := self.(type) {
	case object.Int:
		a = float64(v)
	case object.Float:
		a = float64(v)
	}
	switch v := other.(type) {
	case object.Int:
		b = float64(v)
	case object.Float:
		b = float64(v)
	default:
		return 0, 0, false, ctx.Raise(object.TypeError, "%s can't be coerced into %s", kindName(other), self.Class().Name)
	}
	return a, b, false, nil
}

func installInteger() {
	c := object.IntegerClass
	arith := func(name string, ints func(a, b int64) (object.Value, error), floats func(a, b float64) object.Value) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, b, isInt, err := numArgs(ctx, self, args[0], name)
			if err != nil {
				return nil, err
			}
			if isInt {
				v, err := ints(int64(self.(object.Int)), int64(args[0].(object.Int)))
				if err != nil {
					return nil, ctx.Raise(object.ZeroDivisionError, "%s", err.Error())
				}
				return v, nil
			}
			return floats(a, b), nil
		})
	}
	arith("+", func(a, b int64) (object.Value, error) { return object.IntValue(a + b), nil }, func(a, b float64) object.Value { return object.Float(a + b) })
	arith("-", func(a, b int64) (object.Value, error) { return object.IntValue(a - b), nil }, func(a, b float64) object.Value { return object.Float(a - b) })
	arith("*", func(a, b int64) (object.Value, error) { return object.IntValue(a * b), nil }, func(a, b float64) object.Value { return object.Float(a * b) })
	arith("/", func(a, b int64) (object.Value, error) {
		if b == 0 {
			return nil, errDivZero
		}
		return object.IntValue(floorDiv(a, b)), nil
	}, func(a, b float64) object.Value { return object.Float(a / b) })
	arith("%", func(a, b int64) (object.Value, error) {
		if b == 0 {
			return nil, errDivZero
		}
		return object.IntValue(floorMod(a, b)), nil
	}, func(a, b float64) object.Value { return object.Float(floatMod(a, b)) })
	def(c, "modulo", 1, c.Methods["%"].Native)
	def(c, "div", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "div")
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return nil, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return object.IntValue(int64(math.Floor(a / b))), nil
	})
	def(c, "fdiv", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "fdiv")
		if err != nil {
			return nil, err
		}
		return object.Float(a / b), nil
	})
	def(c, "divmod", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, isInt, err := numArgs(ctx, self, args[0], "divmod")
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return nil, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		if isInt {
			ai, bi := int64(self.(object.Int)), int64(args[0].(object.Int))
			return object.NewList(object.IntValue(floorDiv(ai, bi)), object.IntValue(floorMod(ai, bi))), nil
		}
		q := math.Floor(a / b)
		return object.NewList(object.Float(q), object.Float(a-q*b)), nil
	})
	def(c, "**", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, isInt, err := numArgs(ctx, self, args[0], "**")
		if err != nil {
			return nil, err
		}
		if isInt && b >= 0 {
			return object.IntValue(ipow(int64(a), int64(b))), nil
		}
		return object.Float(math.Pow(a, b)), nil
	})
	def(c, "pow", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 2 {
			b, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			m, err := wantInt(ctx, args[1], "")
			if err != nil {
				return nil, err
			}
			return object.IntValue(powMod(int64(self.(object.Int)), b, m)), nil
		}
		if len(args) != 1 {
			return nil, argError(ctx, len(args), "1..2")
		}
		return c.Methods["**"].Native(ctx, self, args, kw, block)
	})
	def(c, "-@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return -self.(object.Int), nil
	})
	def(c, "+@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "~", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ^self.(object.Int), nil
	})
	bitwise := func(name string, f func(a, b int64) int64) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			b, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			return object.IntValue(f(int64(self.(object.Int)), b)), nil
		})
	}
	bitwise("&", func(a, b int64) int64 { return a & b })
	bitwise("|", func(a, b int64) int64 { return a | b })
	bitwise("^", func(a, b int64) int64 { return a ^ b })
	bitwise(">>", func(a, b int64) int64 {
		if b < 0 {
			return a << uint(-b)
		}
		return a >> uint(b)
	})
	bitwise("[]", func(a, b int64) int64 { return (a >> uint(b)) & 1 })
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].(type) {
		case object.Int, object.Float:
			r, err := ctx.Compare(self, args[0])
			return object.IntValue(r), err
		}
		return object.Nil, nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Equal(self, args[0])), nil
	})
	def(c, "abs", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if v := self.(object.Int); v < 0 {
			return -v, nil
		}
		return self, nil
	})
	def(c, "magnitude", 0, c.Methods["abs"].Native)
	pred := func(name string, f func(int64) bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.BoolOf(f(int64(self.(object.Int)))), nil
		})
	}
	pred("zero?", func(i int64) bool { return i == 0 })
	pred("even?", func(i int64) bool { return i%2 == 0 })
	pred("odd?", func(i int64) bool { return i%2 != 0 })
	pred("positive?", func(i int64) bool { return i > 0 })
	pred("negative?", func(i int64) bool { return i < 0 })
	pred("integer?", func(int64) bool { return true })
	pred("finite?", func(int64) bool { return true })
	pred("nan?", func(int64) bool { return false })
	pred("infinite?", func(int64) bool { return false })
	def(c, "nonzero?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.(object.Int) == 0 {
			return object.Nil, nil
		}
		return self, nil
	})
	def(c, "succ", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.(object.Int) + 1, nil
	})
	def(c, "next", 0, c.Methods["succ"].Native)
	def(c, "pred", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.(object.Int) - 1, nil
	})
	def(c, "times", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := int64(self.(object.Int))
		if block == nil {
			elems := []object.Value{}
			for i := int64(0); i < n; i++ {
				elems = append(elems, object.IntValue(i))
			}
			return object.NewList(elems...), nil
		}
		for i := int64(0); i < n; i++ {
			if _, err := ctx.Yield(block, object.IntValue(i)); err != nil {
				return nil, err
			}
		}
		return self, nil
	})
	step := func(name string, dir int64) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			to, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			from := int64(self.(object.Int))
			var elems []object.Value
			for i := from; dir > 0 && i <= to || dir < 0 && i >= to; i += dir {
				if block == nil {
					elems = append(elems, object.IntValue(i))
					continue
				}
				if _, err := ctx.Yield(block, object.IntValue(i)); err != nil {
					return nil, err
				}
			}
			if block == nil {
				return object.NewList(elems...), nil
			}
			return self, nil
		})
	}
	step("upto", 1)
	step("downto", -1)
	def(c, "step", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 2 {
			return nil, argError(ctx, len(args), "2")
		}
		to, err := wantInt(ctx, args[0], "")
		if err != nil {
			return nil, err
		}
		by, err := wantInt(ctx, args[1], "")
		if err != nil || by == 0 {
			return nil, ctx.Raise(object.ArgumentError, "step can't be 0")
		}
		var elems []object.Value
		for i := int64(self.(object.Int)); by > 0 && i <= to || by < 0 && i >= to; i += by {
			if block == nil {
				elems = append(elems, object.IntValue(i))
				continue
			}
			if _, err := ctx.Yield(block, object.IntValue(i)); err != nil {
				return nil, err
			}
		}
		if block == nil {
			return object.NewList(elems...), nil
		}
		return self, nil
	})
	def(c, "to_s", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		base := int64(10)
		if len(args) == 1 {
			b, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			if b < 2 || b > 36 {
				return nil, ctx.Raise(object.ArgumentError, "invalid radix %d", b)
			}
			base = b
		}
		return object.String(strconv.FormatInt(int64(self.(object.Int)), int(base))), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(strconv.FormatInt(int64(self.(object.Int)), 10)), nil
	})
	def(c, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "to_int", 0, c.Methods["to_i"].Native)
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(self.(object.Int)), nil
	})
	def(c, "chr", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v := int64(self.(object.Int))
		if v < 0 || v > 0x10ffff {
			return nil, ctx.Raise(object.RangeError, "%d out of char range", v)
		}
		return object.String(string(rune(v))), nil
	})
	def(c, "ord", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(hashString(object.Key(self))), nil
	})
	def(c, "gcd", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b, err := wantInt(ctx, args[0], "")
		if err != nil {
			return nil, err
		}
		return object.IntValue(gcd(int64(self.(object.Int)), b)), nil
	})
	def(c, "lcm", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b, err := wantInt(ctx, args[0], "")
		if err != nil {
			return nil, err
		}
		a := int64(self.(object.Int))
		if a == 0 || b == 0 {
			return object.IntValue(0), nil
		}
		return object.IntValue(abs64(a / gcd(a, b) * b)), nil
	})
	def(c, "digits", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v := int64(self.(object.Int))
		if v < 0 {
			return nil, ctx.Raise(object.RangeError, "out of domain")
		}
		elems := []object.Value{}
		for {
			elems = append(elems, object.IntValue(v%10))
			v /= 10
			if v == 0 {
				break
			}
		}
		return object.NewList(elems...), nil
	})
	def(c, "bit_length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v := int64(self.(object.Int))
		if v < 0 {
			v = ^v
		}
		n := 0
		for v != 0 {
			n++
			v >>= 1
		}
		return object.IntValue(n), nil
	})
	rounding := func(name string, f func(float64) float64) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) == 0 {
				return self, nil
			}
			digits, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			if digits >= 0 {
				return self, nil
			}
			p := math.Pow(10, float64(-digits))
			return object.IntValue(int64(f(float64(self.(object.Int))/p) * p)), nil
		})
	}
	rounding("round", math.Round)
	rounding("floor", math.Floor)
	rounding("ceil", math.Ceil)
	rounding("truncate", math.Trunc)
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(8), nil
	})
}

var errDivZero = divZero{}

type divZero struct{}

func (divZero) Error() string { return "divided by 0" }

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func floorMod(a, b int64) int64 {
	m := a % b
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

func floatMod(a, b float64) float64 {
	m := math.Mod(a, b)
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

func ipow(a, b int64) int64 {
	result := int64(1)
	for b > 0 {
		if b&1 == 1 {
			result *= a
		}
		a *= a
		b >>= 1
	}
	return result
}

func powMod(a, b, m int64) int64 {
	result := int64(1)
	a = floorMod(a, m)
	for b > 0 {
		if b&1 == 1 {
			result = floorMod(result*a, m)
		}
		a = floorMod(a*a, m)
		b >>= 1
	}
	return result
}

func gcd(a, b int64) int64 {
	a, b = abs64(a), abs64(b)
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs64(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

func installFloat() {
	c := object.FloatClass
	arith := func(name string, f func(a, b float64) float64) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, b, _, err := numArgs(ctx, self, args[0], name)
			if err != nil {
				return nil, err
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
			return nil, err
		}
		if b == 0 {
			return nil, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return object.IntValue(int64(math.Floor(a / b))), nil
	})
	def(c, "divmod", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		a, b, _, err := numArgs(ctx, self, args[0], "divmod")
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return nil, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		q := math.Floor(a / b)
		return object.NewList(object.Float(q), object.Float(a-q*b)), nil
	})
	def(c, "-@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return -self.(object.Float), nil
	})
	def(c, "+@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].(type) {
		case object.Int, object.Float:
			r, err := ctx.Compare(self, args[0])
			return object.IntValue(r), err
		}
		return object.Nil, nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Equal(self, args[0])), nil
	})
	def(c, "abs", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(math.Abs(float64(self.(object.Float)))), nil
	})
	def(c, "magnitude", 0, c.Methods["abs"].Native)
	pred := func(name string, f func(float64) bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.BoolOf(f(float64(self.(object.Float)))), nil
		})
	}
	pred("zero?", func(f float64) bool { return f == 0 })
	pred("positive?", func(f float64) bool { return f > 0 })
	pred("negative?", func(f float64) bool { return f < 0 })
	pred("nan?", math.IsNaN)
	pred("finite?", func(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) })
	pred("integer?", func(float64) bool { return false })
	def(c, "infinite?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f := float64(self.(object.Float))
		switch {
		case math.IsInf(f, 1):
			return object.IntValue(1), nil
		case math.IsInf(f, -1):
			return object.IntValue(-1), nil
		}
		return object.Nil, nil
	})
	def(c, "nonzero?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.(object.Float) == 0 {
			return object.Nil, nil
		}
		return self, nil
	})
	rounding := func(name string, f func(float64) float64) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			x := float64(self.(object.Float))
			if len(args) == 0 {
				if math.IsNaN(x) || math.IsInf(x, 0) {
					return nil, ctx.Raise(object.RangeError, "%s", object.FormatFloat(x))
				}
				return object.IntValue(int64(f(x))), nil
			}
			digits, err := wantInt(ctx, args[0], "")
			if err != nil {
				return nil, err
			}
			p := math.Pow(10, float64(digits))
			r := f(x*p) / p
			if digits <= 0 {
				return object.IntValue(int64(r)), nil
			}
			return object.Float(r), nil
		})
	}
	rounding("round", roundHalfAway)
	rounding("floor", math.Floor)
	rounding("ceil", math.Ceil)
	rounding("truncate", math.Trunc)
	def(c, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		x := float64(self.(object.Float))
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, ctx.Raise(object.RangeError, "%s", object.FormatFloat(x))
		}
		return object.IntValue(int64(x)), nil
	})
	def(c, "to_int", 0, c.Methods["to_i"].Native)
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(object.FormatFloat(float64(self.(object.Float)))), nil
	})
	def(c, "inspect", 0, c.Methods["to_s"].Native)
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(hashString(object.Key(self))), nil
	})
	object.ObjectClass.Consts["Float"] = c
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
