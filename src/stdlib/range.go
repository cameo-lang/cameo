package stdlib

import (
	"math"

	"github.com/radiospiel/amber/src/object"
)

func installRange() {
	c := object.RangeClass
	r := func(self object.Value) *object.Range { return self.Range() }
	def(c, "first", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			if r(self).Lo.IsZero() {
				return object.Value{}, ctx.Raise(object.RangeError, "cannot get the first element of beginless range")
			}
			return r(self).Lo, nil
		}
		return object.Enumerable.Methods["first"].Native(ctx, self, args, kw, block)
	})
	def(c, "begin", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r(self).Lo.IsZero() {
			return object.Nil, nil
		}
		return r(self).Lo, nil
	})
	def(c, "end", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r(self).Hi.IsZero() {
			return object.Nil, nil
		}
		return r(self).Hi, nil
	})
	def(c, "last", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			if r(self).Hi.IsZero() {
				return object.Value{}, ctx.Raise(object.RangeError, "cannot get the last element of endless range")
			}
			return r(self).Hi, nil
		}
		elems, err := rangeElems(ctx, r(self))
		if err != nil {
			return object.Value{}, err
		}
		n, err := countArg(ctx, args, len(elems))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems[len(elems)-n:]...)...), nil
	})
	def(c, "exclude_end?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(r(self).Exclusive), nil
	})
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, ok1 := r(self).Lo.AsInt()
		hi, ok2 := r(self).Hi.AsInt()
		if !ok1 {
			return object.Value{}, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r(self).Lo)))
		}
		if !ok2 {
			if r(self).Hi.IsZero() {
				return object.Float(math.Inf(1)), nil
			}
			return object.Nil, nil
		}
		n := int64(hi - lo + 1)
		if r(self).Exclusive {
			n--
		}
		if n < 0 {
			n = 0
		}
		return object.Int(n), nil
	})
	def(c, "count", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() {
			return c.Methods["size"].Native(ctx, self, nil, nil, object.Value{})
		}
		return object.Enumerable.Methods["count"].Native(ctx, self, args, kw, block)
	})
	cover := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in, err := rangeIncludes(ctx, r(self), args[0])
		return object.Bool(in), err
	}
	def(c, "include?", 1, cover)
	def(c, "member?", 1, cover)
	def(c, "cover?", 1, cover)
	def(c, "===", 1, cover)
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		lo, ok := r(self).Lo.AsInt()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r(self).Lo)))
		}
		if r(self).Hi.IsZero() {
			for i := lo; ; i++ {
				if _, err := ctx.Yield(block, object.Int(i)); err != nil {
					return object.Value{}, err
				}
			}
		}
		elems, err := rangeElems(ctx, r(self))
		if err != nil {
			return object.Value{}, err
		}
		for _, e := range elems {
			if _, err := ctx.Yield(block, e); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "step", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		by, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if by <= 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "step can't be negative or 0")
		}
		elems, err := rangeElems(ctx, r(self))
		if err != nil {
			return object.Value{}, err
		}
		out := []object.Value{}
		for i := 0; i < len(elems); i += int(by) {
			out = append(out, elems[i])
		}
		return yieldEach(ctx, out, block)
	})
	def(c, "%", 1, c.Methods["step"].Native)
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems, err := rangeElems(ctx, r(self))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems...)...), nil
	})
	def(c, "to_list", 0, c.Methods["to_a"].Native)
	def(c, "sum", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, ok1 := r(self).Lo.AsInt()
		hi, ok2 := r(self).Hi.AsInt()
		if ok1 && ok2 && block.IsZero() && len(args) == 0 {
			if r(self).Exclusive {
				hi--
			}
			if hi < lo {
				return object.Int(0), nil
			}
			return object.Int((int64(lo) + int64(hi)) * (int64(hi) - int64(lo) + 1) / 2), nil
		}
		return object.Enumerable.Methods["sum"].Native(ctx, self, args, kw, block)
	})
	def(c, "min", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() && !r(self).Lo.IsZero() {
			if size, err := c.Methods["size"].Native(ctx, self, nil, nil, object.Value{}); err == nil {
				if n, ok := size.AsInt(); ok && n == 0 {
					return object.Nil, nil
				}
			}
			return r(self).Lo, nil
		}
		return object.Enumerable.Methods["min"].Native(ctx, self, args, kw, block)
	})
	def(c, "max", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() {
			if hi, ok := r(self).Hi.AsInt(); ok {
				if r(self).Exclusive {
					hi--
				}
				if lo, ok := r(self).Lo.AsInt(); ok && hi < lo {
					return object.Nil, nil
				}
				return object.Int(hi), nil
			}
		}
		return object.Enumerable.Methods["max"].Native(ctx, self, args, kw, block)
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, hi := "", ""
		var err error
		if !r(self).Lo.IsZero() {
			if lo, err = ctx.ToS(r(self).Lo); err != nil {
				return object.Value{}, err
			}
		}
		if !r(self).Hi.IsZero() {
			if hi, err = ctx.ToS(r(self).Hi); err != nil {
				return object.Value{}, err
			}
		}
		if r(self).Exclusive {
			return object.String(lo + "..." + hi), nil
		}
		return object.String(lo + ".." + hi), nil
	})
}

// rangeIncludes is Range#include? and `range === v`.
func rangeIncludes(ctx object.Context, r *object.Range, v object.Value) (bool, error) {
	if !r.Lo.IsZero() {
		c, err := ctx.Compare(r.Lo, v)
		if err != nil {
			return false, nil
		}
		if c > 0 {
			return false, nil
		}
	}
	if !r.Hi.IsZero() {
		c, err := ctx.Compare(v, r.Hi)
		if err != nil {
			return false, nil
		}
		if c > 0 || (c == 0 && r.Exclusive) {
			return false, nil
		}
	}
	return true, nil
}

// rangeElems expands an integer range into its elements.
func rangeElems(ctx object.Context, r *object.Range) ([]object.Value, error) {
	lo, ok1 := r.Lo.AsInt()
	hi, ok2 := r.Hi.AsInt()
	if !ok1 || !ok2 {
		if r.Hi.IsZero() && ok1 {
			return nil, ctx.Raise(object.RangeError, "cannot convert endless range to a list")
		}
		return nil, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r.Lo)))
	}
	if r.Exclusive {
		hi--
	}
	out := []object.Value{}
	for i := lo; i <= hi; i++ {
		out = append(out, object.Int(i))
	}
	return out, nil
}
