package stdlib

import (
	"github.com/cameo-lang/cameo/src/object"
)

// installNumericGeneric defines the methods Ruby's Numeric gives a number
// class of a script's own, `class Money < Numeric`, each through the
// instance's own operators, as numeric.c does: zero? asks ==, positive?
// asks <=>, -@ asks coerce and -, floor converts with to_f. Integer and
// Float define their own of each, and so does BigDecimal of most, so these
// run for an instance alone, or for a builtin number where the builtin
// class has none (real?).
func installNumericGeneric() {
	n := object.NumericClass
	def(n, "zero?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, object.Int(0))
		return object.Bool(eq), err
	})
	def(n, "positive?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, err := ctx.Compare(self, object.Int(0))
		return object.Bool(c > 0), err
	})
	def(n, "negative?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, err := ctx.Compare(self, object.Int(0))
		return object.Bool(c < 0), err
	})
	// -@ is 0 - self, with 0 in the receiver's kind: coerce(0) gives the
	// pair [0, self] in a kind both share.
	def(n, "-@", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		pair, err := coercePair(ctx, self, object.Int(0))
		if err != nil {
			return object.Value{}, err
		}
		return ctx.Call(pair[0], "-", pair[1])
	})
	abs := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, err := ctx.Compare(self, object.Int(0))
		if err != nil || c >= 0 {
			return self, err
		}
		return ctx.Call(self, "-@")
	}
	def(n, "abs", 0, abs)
	def(n, "magnitude", 0, abs)
	def(n, "integer?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.IsInteger()), nil
	})
	def(n, "real?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	def(n, "finite?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	def(n, "infinite?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	// coerce is Ruby's default: a value of the receiver's own class as it
	// is, anything else as a Float beside the receiver as one.
	def(n, "coerce", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].Class() == self.Class() {
			return object.NewList(args[0], self), nil
		}
		other, err := toFloat(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		me, err := toFloat(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(other, me), nil
	})
	// div is (self / other).floor, and refuses a zero divisor first.
	div := func(ctx object.Context, self, other object.Value) (object.Value, error) {
		if zero, err := ctx.Equal(object.Int(0), other); err != nil || zero {
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		q, err := ctx.Call(self, "/", other)
		if err != nil {
			return object.Value{}, err
		}
		return ctx.Call(q, "floor")
	}
	// mod is self - other * self.div(other).
	mod := func(ctx object.Context, self, other object.Value) (object.Value, error) {
		q, err := div(ctx, self, other)
		if err != nil {
			return object.Value{}, err
		}
		p, err := ctx.Call(other, "*", q)
		if err != nil {
			return object.Value{}, err
		}
		return ctx.Call(self, "-", p)
	}
	def(n, "div", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return div(ctx, self, args[0])
	})
	modulo := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return mod(ctx, self, args[0])
	}
	def(n, "modulo", 1, modulo)
	def(n, "%", 1, modulo)
	def(n, "divmod", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		q, err := div(ctx, self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		r, err := mod(ctx, self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(q, r), nil
	})
	// remainder is the modulo with the receiver's sign.
	def(n, "remainder", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		r, err := mod(ctx, self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if zero, err := ctx.Equal(r, object.Int(0)); err != nil || zero {
			return r, err
		}
		a, err := ctx.Compare(self, object.Int(0))
		if err != nil {
			return object.Value{}, err
		}
		b, err := ctx.Compare(args[0], object.Int(0))
		if err != nil {
			return object.Value{}, err
		}
		if a < 0 && b > 0 || a > 0 && b < 0 {
			return ctx.Call(r, "-", args[0])
		}
		return r, nil
	})
	def(n, "fdiv", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f, err := toFloat(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return ctx.Call(f, "/", args[0])
	})
	// floor, ceil, round and truncate are the Float's, digits and all.
	for _, name := range []string{"floor", "ceil", "round", "truncate"} {
		def(n, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			f, err := toFloat(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return ctx.CallWith(f, name, args, kw, object.Value{})
		})
	}
	def(n, "to_int", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Call(self, "to_i")
	})
}

// coercePair is self.coerce(other) read as its pair: [other, self] in a
// kind both share.
func coercePair(ctx object.Context, self, other object.Value) ([2]object.Value, error) {
	r, err := ctx.Call(self, "coerce", other)
	if err != nil {
		return [2]object.Value{}, err
	}
	l, ok := r.AsList()
	if !ok || l.Len() != 2 {
		return [2]object.Value{}, ctx.Raise(object.TypeError, "coerce must return [x, y]")
	}
	return [2]object.Value{l.Elems[0], l.Elems[1]}, nil
}

// numericNonzero is Numeric#nonzero?: nil when the receiver is zero, the
// receiver otherwise. A builtin number answers itself; a number class of a
// script's own answers through its zero?.
func numericNonzero(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if f, ok := self.AsNumber(); ok {
		if f == 0 {
			return object.Nil, nil
		}
		return self, nil
	}
	zero, err := ctx.Call(self, "zero?")
	if err != nil || zero.Truthy() {
		return object.Nil, err
	}
	return self, nil
}

// instanceStep is Numeric#step for a number class of a script's own:
// self, self + by, and so on while the value has not passed limit, through
// the class's own + and <=>, as Ruby's num_step does for a value that is no
// Integer or Float. Without a block it is the Enumerator over the same.
func instanceStep(ctx object.Context, self, limit, by object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if block.IsZero() {
		return ctx.CallWith(self, "to_enum", append([]object.Value{object.Symbol("step")}, args...), kw, object.Value{})
	}
	if by.IsNil() {
		by = object.Int(1)
	}
	if zero, err := ctx.Equal(by, object.Int(0)); err != nil || zero {
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, ctx.Raise(object.ArgumentError, "step can't be 0")
	}
	down, err := ctx.Compare(by, object.Int(0))
	if err != nil {
		return object.Value{}, err
	}
	for v := self; ; {
		if !limit.IsNil() {
			c, err := ctx.Compare(v, limit)
			if err != nil {
				return object.Value{}, err
			}
			if down > 0 && c > 0 || down < 0 && c < 0 {
				return self, nil
			}
		}
		if _, err := ctx.Yield(block, v); err != nil {
			return object.Value{}, err
		}
		if v, err = ctx.Call(v, "+", by); err != nil {
			return object.Value{}, err
		}
	}
}
