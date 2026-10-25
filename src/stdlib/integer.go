package stdlib

import (
	"math"
	"math/big"
	"strconv"

	"github.com/cameo-lang/cameo/src/object"
)

// An Integer comes in two forms, a KindInt and, beyond 64 bits, a KindBig.
// Each method takes the 64-bit path when it can and big.Int arithmetic
// otherwise; object.BigInt brings a result that fits back to a KindInt.

// bigOf reads an integer in either form; a KindBig's digits are its own and
// must not be changed.
func bigOf(v object.Value) *big.Int {
	b, _ := v.AsBigInt()
	return b
}

// floatToInteger is the integer part of a float, and a FloatDomainError
// for NaN and the infinities, which have none.
func floatToInteger(ctx object.Context, f float64) (object.Value, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return object.Value{}, ctx.Raise(object.FloatDomainError, "%s", object.FormatFloat(f))
	}
	return object.FloatToInteger(f), nil
}

// compareNumber is Integer and Float's <=>: exact, and nil for a value
// that is not a number, or NaN.
func compareNumber(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if r, ok := object.CompareNumbers(self, args[0]); ok {
		return object.Int(r), nil
	}
	return object.Nil, nil
}

func installInteger() {
	c := object.IntegerClass
	// arith defines a binary operator: small on two KindInts, which reports
	// overflow, large on any two integers, floats when either is a float.
	// A zero integer divisor raises before either runs, where zero says so.
	arith := func(name string, zero bool, small func(a, b int64) (int64, bool), large func(a, b *big.Int) *big.Int, floats func(a, b float64) object.Value) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			other := args[0]
			if other.IsInteger() {
				if x, ok := self.AsInt(); ok {
					if y, ok := other.AsInt(); ok {
						if zero && y == 0 {
							return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
						}
						if r, ok := small(x, y); ok {
							return object.Int(r), nil
						}
					}
				}
				if zero && bigOf(other).Sign() == 0 {
					return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
				}
				return object.BigInt(large(bigOf(self), bigOf(other))), nil
			}
			a, b, _, err := numArgs(ctx, self, other, name)
			if err != nil {
				return object.Value{}, err
			}
			return floats(a, b), nil
		})
	}
	arith("+", false, object.AddInt, func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) }, func(a, b float64) object.Value { return object.Float(a + b) })
	arith("-", false, object.SubInt, func(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) }, func(a, b float64) object.Value { return object.Float(a - b) })
	arith("*", false, object.MulInt, func(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) }, func(a, b float64) object.Value { return object.Float(a * b) })
	arith("/", true, func(a, b int64) (int64, bool) {
		if a == math.MinInt64 && b == -1 {
			return 0, false
		}
		return floorDiv(a, b), true
	}, func(a, b *big.Int) *big.Int {
		q, _ := floorDivMod(a, b)
		return q
	}, func(a, b float64) object.Value { return object.Float(a / b) })
	arith("%", true, func(a, b int64) (int64, bool) { return floorMod(a, b), true }, func(a, b *big.Int) *big.Int {
		_, m := floorDivMod(a, b)
		return m
	}, func(a, b float64) object.Value { return object.Float(floatMod(a, b)) })
	def(c, "modulo", 1, c.Methods["%"].Native)
	def(c, "div", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsInteger() {
			return c.Methods["/"].Native(ctx, self, args, kw, block)
		}
		a, b, _, err := numArgs(ctx, self, args[0], "div")
		if err != nil {
			return object.Value{}, err
		}
		if b == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		return floatToInteger(ctx, math.Floor(a/b))
	})
	def(c, "fdiv", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// Two integers divide exactly, then round once, as Ruby's do; an
		// integer beyond 2**53 would round before the division otherwise.
		if args[0].IsInteger() && bigOf(args[0]).Sign() != 0 && (exceeds53(self) || exceeds53(args[0])) {
			f, _ := new(big.Rat).SetFrac(bigOf(self), bigOf(args[0])).Float64()
			return object.Float(f), nil
		}
		a, b, _, err := numArgs(ctx, self, args[0], "fdiv")
		if err != nil {
			return object.Value{}, err
		}
		return object.Float(a / b), nil
	})
	def(c, "divmod", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsInteger() {
			q, err := c.Methods["/"].Native(ctx, self, args, kw, block)
			if err != nil {
				return object.Value{}, err
			}
			m, err := c.Methods["%"].Native(ctx, self, args, kw, block)
			if err != nil {
				return object.Value{}, err
			}
			return object.NewList(q, m), nil
		}
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
	def(c, "**", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if e, ok := args[0].AsInt(); ok && e >= 0 {
			if x, ok := self.AsInt(); ok {
				if r, ok := ipow(x, e); ok {
					return object.Int(r), nil
				}
			}
			// Past 32M bits, CRuby gives up on the integer and answers
			// with a float, Infinity; so does cameo.
			if x := bigOf(self); int64(x.BitLen()) <= bigPowBits/e || x.BitLen() <= 1 {
				return object.BigInt(new(big.Int).Exp(x, big.NewInt(e), nil)), nil
			}
		}
		a, b, _, err := numArgs(ctx, self, args[0], "**")
		if err != nil {
			return object.Value{}, err
		}
		return object.Float(floatPow(a, b)), nil
	})
	def(c, "pow", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 2 {
			if !args[0].IsInteger() || !args[1].IsInteger() {
				return object.Value{}, ctx.Raise(object.TypeError, "Integer#pow() 2nd argument not allowed unless all arguments are integers")
			}
			if x, ok := self.AsInt(); ok {
				e, eok := args[0].AsInt()
				m, mok := args[1].AsInt()
				if eok && mok && e >= 0 && m != 0 && m > -(1<<31) && m < 1<<31 {
					return object.Int(powMod(x, e, m)), nil
				}
			}
			e, m := bigOf(args[0]), bigOf(args[1])
			if e.Sign() < 0 {
				return object.Value{}, ctx.Raise(object.RangeError, "Integer#pow() 1st argument cannot be negative when 2nd argument specified")
			}
			if m.Sign() == 0 {
				return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
			}
			abs := new(big.Int).Abs(m)
			_, base := floorDivMod(bigOf(self), abs)
			_, r := floorDivMod(new(big.Int).Exp(base, e, abs), m)
			return object.BigInt(r), nil
		}
		if len(args) != 1 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		return c.Methods["**"].Native(ctx, self, args, kw, block)
	})
	// unary defines a method of self alone, small on a KindInt where it
	// does not overflow.
	unary := func(name string, small func(int64) (int64, bool), large func(*big.Int) *big.Int) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if x, ok := self.AsInt(); ok {
				if r, ok := small(x); ok {
					return object.Int(r), nil
				}
			}
			return object.BigInt(large(bigOf(self))), nil
		})
	}
	unary("-@", func(x int64) (int64, bool) { return -x, x != math.MinInt64 }, func(b *big.Int) *big.Int { return new(big.Int).Neg(b) })
	unary("~", func(x int64) (int64, bool) { return ^x, true }, func(b *big.Int) *big.Int { return new(big.Int).Not(b) })
	unary("abs", func(x int64) (int64, bool) {
		if x < 0 {
			return -x, x != math.MinInt64
		}
		return x, true
	}, func(b *big.Int) *big.Int { return new(big.Int).Abs(b) })
	def(c, "magnitude", 0, c.Methods["abs"].Native)
	unary("succ", func(x int64) (int64, bool) { return object.AddInt(x, 1) }, func(b *big.Int) *big.Int { return new(big.Int).Add(b, big.NewInt(1)) })
	def(c, "next", 0, c.Methods["succ"].Native)
	unary("pred", func(x int64) (int64, bool) { return object.SubInt(x, 1) }, func(b *big.Int) *big.Int { return new(big.Int).Sub(b, big.NewInt(1)) })
	def(c, "+@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	bitwise := func(name string, small func(a, b int64) int64, large func(a, b *big.Int) *big.Int) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if !args[0].IsInteger() {
				_, err := wantInt(ctx, args[0], "")
				return object.Value{}, err
			}
			x, xok := self.AsInt()
			y, yok := args[0].AsInt()
			if xok && yok {
				return object.Int(small(x, y)), nil
			}
			return object.BigInt(large(bigOf(self), bigOf(args[0]))), nil
		})
	}
	bitwise("&", func(a, b int64) int64 { return a & b }, func(a, b *big.Int) *big.Int { return new(big.Int).And(a, b) })
	bitwise("|", func(a, b int64) int64 { return a | b }, func(a, b *big.Int) *big.Int { return new(big.Int).Or(a, b) })
	bitwise("^", func(a, b int64) int64 { return a ^ b }, func(a, b *big.Int) *big.Int { return new(big.Int).Xor(a, b) })
	// shift is << for a positive count and >> for a negative one; a left
	// shift that overflows moves to a big integer.
	shift := func(ctx object.Context, self object.Value, by object.Value, left bool) (object.Value, error) {
		n, err := wantInt(ctx, by, "")
		if err != nil {
			return object.Value{}, err
		}
		if !left {
			n = -n
		}
		if x, ok := self.AsInt(); ok {
			switch {
			case n <= 0 && n > -64:
				return object.Int(x >> uint(-n)), nil
			case n <= 0 && x < 0:
				return object.Int(-1), nil
			case n <= 0:
				return object.Int(0), nil
			case n < 63 && (x<<uint(n))>>uint(n) == x:
				return object.Int(x << uint(n)), nil
			}
		}
		if n >= 0 {
			if b := bigOf(self); b.Sign() != 0 && int64(b.BitLen())+n > maxShiftBits {
				return object.Value{}, ctx.Raise(object.RangeError, "shift width too big")
			}
			return object.BigInt(new(big.Int).Lsh(bigOf(self), uint(n))), nil
		}
		return object.BigInt(new(big.Int).Rsh(bigOf(self), uint(-n))), nil
	}
	def(c, "<<", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return shift(ctx, self, args[0], true)
	})
	def(c, ">>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return shift(ctx, self, args[0], false)
	})
	def(c, "[]", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 {
			return object.Int(0), nil
		}
		if x, ok := self.AsInt(); ok {
			if i > 63 {
				i = 63
			}
			return object.Int((x >> uint(i)) & 1), nil
		}
		return object.Int(bigOf(self).Bit(int(i))), nil
	})
	def(c, "<=>", 1, compareNumber)
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
	pred := func(name string, f func(sign int, odd bool) bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if x, ok := self.AsInt(); ok {
				sign := 0
				if x > 0 {
					sign = 1
				} else if x < 0 {
					sign = -1
				}
				return object.Bool(f(sign, x%2 != 0)), nil
			}
			b := self.Big()
			return object.Bool(f(b.Sign(), b.Bit(0) == 1)), nil
		})
	}
	pred("zero?", func(sign int, odd bool) bool { return sign == 0 })
	pred("even?", func(sign int, odd bool) bool { return !odd })
	pred("odd?", func(sign int, odd bool) bool { return odd })
	pred("positive?", func(sign int, odd bool) bool { return sign > 0 })
	pred("negative?", func(sign int, odd bool) bool { return sign < 0 })
	pred("integer?", func(int, bool) bool { return true })
	pred("finite?", func(int, bool) bool { return true })
	pred("nan?", func(int, bool) bool { return false })
	// infinite? is nil for an integer, which is never infinite, as Float's
	// is nil for a finite float.
	def(c, "infinite?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	def(c, "nonzero?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if x, ok := self.AsInt(); ok && x == 0 {
			return object.Nil, nil
		}
		return self, nil
	})
	def(c, "times", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := wantInt(ctx, self, "")
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			elems := []object.Value{}
			for i := int64(0); i < n; i++ {
				elems = append(elems, object.Int(i))
			}
			return object.NewList(elems...), nil
		}
		for i := int64(0); i < n; i++ {
			if _, err := ctx.Yield(block, object.Int(i)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	bits := func(name string, f func(and, mask *big.Int) bool) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			mask := args[0]
			if mask.Kind() == object.KindFloat {
				if _, err := floatToInteger(ctx, mask.Float()); err != nil {
					return object.Value{}, err
				}
				mask = object.FloatToInteger(mask.Float())
			}
			if !mask.IsInteger() {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(mask))
			}
			m := bigOf(mask)
			return object.Bool(f(new(big.Int).And(bigOf(self), m), m)), nil
		})
	}
	bits("allbits?", func(and, mask *big.Int) bool { return and.Cmp(mask) == 0 })
	bits("anybits?", func(and, mask *big.Int) bool { return and.Sign() != 0 })
	bits("nobits?", func(and, mask *big.Int) bool { return and.Sign() == 0 })
	step := func(name string, dir int64) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			to := args[0]
			if !to.IsInteger() {
				if to.Kind() != object.KindFloat {
					return object.Value{}, compareError(ctx, self, to)
				}
				to = object.FloatToInteger(math.Floor(to.Float()))
				if dir < 0 {
					to = object.FloatToInteger(math.Ceil(args[0].Float()))
				}
			}
			var elems []object.Value
			err := eachInteger(self, to, dir, func(i object.Value) error {
				if block.IsZero() {
					elems = append(elems, i)
					return nil
				}
				_, err := ctx.Yield(block, i)
				return err
			})
			if err != nil {
				return object.Value{}, err
			}
			if block.IsZero() {
				return object.NewList(elems...), nil
			}
			return self, nil
		})
	}
	step("upto", 1)
	step("downto", -1)
	def(c, "step", -1, numericStep)
	def(c, "to_s", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		base := int64(10)
		if len(args) == 1 {
			b, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			if b < 2 || b > 36 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid radix %d", b)
			}
			base = b
		}
		return object.String(integerText(self, int(base))), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(integerText(self, 10)), nil
	})
	def(c, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "to_int", 0, c.Methods["to_i"].Native)
	def(c, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, _ := self.AsNumber()
		return object.Float(f), nil
	})
	// chr is a byte, US-ASCII below 128 and binary to 255, or given an
	// encoding a character of it.
	def(c, "chr", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := self.AsInt()
		k := object.KindBytes
		if len(args) > 0 {
			var err error
			if k, err = encodingArg(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		} else if ok && v < 0x80 {
			k = object.KindASCII
		}
		limit := int64(0xff)
		switch k {
		case object.KindString:
			limit = 0x10ffff
		case object.KindASCII:
			limit = 0x7f
		}
		if !ok || v < 0 || v > limit || k == object.KindString && v >= 0xd800 && v <= 0xdfff {
			if k == object.KindString && ok && v >= 0 && v <= 0x10ffff {
				return object.Value{}, ctx.Raise(object.RangeError, "invalid codepoint 0x%X in UTF-8", v)
			}
			return object.Value{}, ctx.Raise(object.RangeError, "%s out of char range", integerText(self, 10))
		}
		if k == object.KindString {
			return object.String(string(rune(v))), nil
		}
		return object.StringOf(k, string([]byte{byte(v)})), nil
	})
	def(c, "ord", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	gcdOf := func(ctx object.Context, self, other object.Value) (*big.Int, error) {
		if !other.IsInteger() {
			_, err := wantInt(ctx, other, "")
			if err == nil {
				err = ctx.Raise(object.TypeError, "not an integer")
			}
			return nil, err
		}
		a, b := new(big.Int).Abs(bigOf(self)), new(big.Int).Abs(bigOf(other))
		return new(big.Int).GCD(nil, nil, a, b), nil
	}
	def(c, "gcd", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if x, ok := self.AsInt(); ok && x != math.MinInt64 {
			if y, ok := args[0].AsInt(); ok && y != math.MinInt64 {
				return object.Int(gcd(x, y)), nil
			}
		}
		g, err := gcdOf(ctx, self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.BigInt(g), nil
	})
	def(c, "lcm", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := gcdOf(ctx, self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if g.Sign() == 0 {
			return object.Int(0), nil
		}
		l := new(big.Int).Quo(bigOf(self), g)
		l.Mul(l, bigOf(args[0]))
		return object.BigInt(l.Abs(l)), nil
	})
	def(c, "gcdlcm", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := c.Methods["gcd"].Native(ctx, self, args, kw, block)
		if err != nil {
			return object.Value{}, err
		}
		l, err := c.Methods["lcm"].Native(ctx, self, args, kw, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(g, l), nil
	})
	// remainder is x - y * (x / y).truncate: its sign is the receiver's.
	// Ruby raises for a Float zero only on a bignum receiver.
	def(c, "remainder", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other := args[0]
		if other.IsInteger() {
			if bigOf(other).Sign() == 0 {
				return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
			}
			return object.BigInt(new(big.Int).Rem(bigOf(self), bigOf(other))), nil
		}
		y, ok := other.AsNumber()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "%s can't be coerced into Integer", kindName(other))
		}
		if _, small := self.AsInt(); !small && y == 0 {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		x, _ := self.AsNumber()
		return object.Float(math.Mod(x, y)), nil
	})
	arg := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if bigOf(self).Sign() < 0 {
			return object.Float(math.Pi), nil
		}
		return object.Int(0), nil
	}
	def(c, "arg", 0, arg)
	def(c, "angle", 0, arg)
	def(c, "phase", 0, arg)
	def(c, "digits", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		base := int64(10)
		if len(args) == 1 {
			b, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			if b < 2 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid radix %d", b)
			}
			base = b
		}
		if bigOf(self).Sign() < 0 {
			return object.Value{}, ctx.Raise(object.MathDomainError, "out of domain")
		}
		elems := []object.Value{}
		if v, ok := self.AsInt(); ok {
			for {
				elems = append(elems, object.Int(v%base))
				if v /= base; v == 0 {
					break
				}
			}
			return object.NewList(elems...), nil
		}
		v, b, d := new(big.Int).Set(self.Big()), big.NewInt(base), new(big.Int)
		for v.Sign() > 0 {
			v.QuoRem(v, b, d)
			elems = append(elems, object.Int(d.Int64()))
		}
		return object.NewList(elems...), nil
	})
	def(c, "bit_length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := bigOf(self)
		if b.Sign() < 0 {
			b = new(big.Int).Not(b)
		}
		return object.Int(b.BitLen()), nil
	})
	// rounding to a negative number of digits rounds to a power of ten:
	// down, up, toward zero, or half away from zero.
	rounding := func(name string, f func(q, m, p *big.Int) *big.Int) {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) == 0 {
				return self, nil
			}
			digits, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			if digits >= 0 {
				return self, nil
			}
			p := new(big.Int).Exp(big.NewInt(10), big.NewInt(-digits), nil)
			q, m := floorDivMod(bigOf(self), p)
			return object.BigInt(new(big.Int).Mul(f(q, m, p), p)), nil
		})
	}
	one := big.NewInt(1)
	rounding("floor", func(q, m, p *big.Int) *big.Int { return q })
	rounding("ceil", func(q, m, p *big.Int) *big.Int {
		if m.Sign() != 0 {
			return new(big.Int).Add(q, one)
		}
		return q
	})
	rounding("truncate", func(q, m, p *big.Int) *big.Int {
		if m.Sign() != 0 && q.Sign() < 0 {
			return new(big.Int).Add(q, one)
		}
		return q
	})
	def(c, "round", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		mode, err := roundingMode(ctx, kw)
		if err != nil || len(args) == 0 {
			return self, err
		}
		digits, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		return roundInteger(self, digits, mode), nil
	})
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.Kind() == object.KindInt {
			return object.Int(8), nil
		}
		return object.Int((self.Big().BitLen() + 7) / 8), nil
	})
	def(c, "coerce", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsInteger() {
			return object.NewList(args[0], self), nil
		}
		return coerceFloats(ctx, self, args[0])
	})
	object.ObjectClass.Consts["Integer"] = object.ClassValue(c)
	def(c.Class(), "sqrt", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !args[0].IsInteger() {
			if _, err := wantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
			return floatToInteger(ctx, math.Floor(math.Sqrt(args[0].Float())))
		}
		b := bigOf(args[0])
		if b.Sign() < 0 {
			return object.Value{}, ctx.Raise(object.MathDomainError, "Numerical argument is out of domain - \"isqrt\"")
		}
		return object.BigInt(new(big.Int).Sqrt(b)), nil
	})
}

// addIntegers is a+b for two integers in either form.
func addIntegers(a, b object.Value) object.Value {
	if x, ok := a.AsInt(); ok {
		if y, ok := b.AsInt(); ok {
			if r, ok := object.AddInt(x, y); ok {
				return object.Int(r)
			}
		}
	}
	return object.BigInt(new(big.Int).Add(bigOf(a), bigOf(b)))
}

// exceeds53 reports an integer a float cannot hold exactly.
func exceeds53(v object.Value) bool {
	if x, ok := v.AsInt(); ok {
		return x > 1<<53 || x < -(1<<53)
	}
	return true
}

// integerText prints an integer in a base.
func integerText(v object.Value, base int) string {
	if x, ok := v.AsInt(); ok {
		return strconv.FormatInt(x, base)
	}
	return v.Big().Text(base)
}

// eachInteger calls f with each integer from from to to, counting up for a
// positive dir and down for a negative one, in either form.
func eachInteger(from, to object.Value, dir int64, f func(object.Value) error) error {
	if x, ok := from.AsInt(); ok {
		if y, ok := to.AsInt(); ok && y < math.MaxInt64 && y > math.MinInt64 {
			for i := x; dir > 0 && i <= y || dir < 0 && i >= y; i += dir {
				if err := f(object.Int(i)); err != nil {
					return err
				}
			}
			return nil
		}
	}
	i, end, d := new(big.Int).Set(bigOf(from)), bigOf(to), big.NewInt(dir)
	for {
		c := i.Cmp(end)
		if dir > 0 && c > 0 || dir < 0 && c < 0 {
			return nil
		}
		if err := f(object.BigInt(new(big.Int).Set(i))); err != nil {
			return err
		}
		i.Add(i, d)
	}
}

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

// floorDivMod is Ruby's integer division of big integers: the quotient
// rounded down, and the remainder with the divisor's sign.
func floorDivMod(a, b *big.Int) (q, m *big.Int) {
	q, m = new(big.Int).QuoRem(a, b, new(big.Int))
	if m.Sign() != 0 && (m.Sign() < 0) != (b.Sign() < 0) {
		q.Sub(q, big.NewInt(1))
		m.Add(m, b)
	}
	return q, m
}

// roundInteger rounds an integer to a negative number of digits, a
// multiple of a power of ten, with ties by mode: "up" away from zero,
// "down" toward it, or "even".
func roundInteger(v object.Value, digits int64, mode string) object.Value {
	if digits >= 0 {
		return v
	}
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(-digits), nil)
	// q*p+m with 0 <= m < p.
	q, m := floorDivMod(bigOf(v), p)
	twice := new(big.Int).Lsh(m, 1)
	c := twice.Cmp(p)
	// q is negative exactly when v is, so at a tie q+1 is toward zero
	// for a negative v and away from it otherwise.
	negative := q.Sign() < 0
	if c > 0 || c == 0 && (mode == "up" && !negative || mode == "down" && negative || mode == "even" && q.Bit(0) == 1) {
		q.Add(q, big.NewInt(1))
	}
	return object.BigInt(q.Mul(q, p))
}

func floatMod(a, b float64) float64 {
	m := math.Mod(a, b)
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

// bigPowBits is how large a power ** computes, CRuby's BIGLEN_LIMIT;
// maxShiftBits is how large a shift makes an integer.
const (
	bigPowBits   = 32 * 1024 * 1024
	maxShiftBits = 1 << 31
)

// powMod is a**b mod m with the modulus's sign, for |m| < 2**31, where
// no product leaves 64 bits.
func powMod(a, b, m int64) int64 {
	result := floorMod(1, m)
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

// ipow is a**b for b >= 0, and false when it overflows 64 bits.
func ipow(a, b int64) (int64, bool) {
	result := int64(1)
	for {
		if b&1 == 1 {
			var ok bool
			if result, ok = object.MulInt(result, a); !ok {
				return 0, false
			}
		}
		if b >>= 1; b == 0 {
			return result, true
		}
		var ok bool
		if a, ok = object.MulInt(a, a); !ok {
			return 0, false
		}
	}
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

// compareError is Ruby's rb_cmperr: the ArgumentError of a failed
// comparison, which names b by its inspect when b is nil, a boolean, a
// Symbol or a number, and by its class otherwise.
func compareError(ctx object.Context, a, b object.Value) error {
	name := b.Class().Name
	switch b.Kind() {
	case object.KindNil, object.KindBool, object.KindSymbol, object.KindInt, object.KindFloat:
		s, err := ctx.Inspect(b)
		if err != nil {
			return err
		}
		name = s
	}
	return ctx.Raise(object.ArgumentError, "comparison of %s with %s failed", a.Class().Name, name)
}
