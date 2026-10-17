package stdlib

import (
	"iter"
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
		// An endless range has a first n all the same.
		if hi := object.OrNil(r(self).Hi); hi.IsNil() && len(args) == 1 {
			n, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			if n < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative array size (or size too big)")
			}
			out := []object.Value{}
			if lo, ok := r(self).Lo.AsInt(); ok {
				for i := int64(0); i < n; i++ {
					out = append(out, object.Int(lo+i))
				}
				return object.NewList(out...), nil
			}
			if seq, sym, _, ok := rangeStrings(r(self)); ok {
				for s := range seq {
					if int64(len(out)) == n {
						break
					}
					out = append(out, strValue(s, sym))
				}
				return object.NewList(out...), nil
			}
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
		elems, err := RangeElems(ctx, r(self))
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
			if _, _, _, ok := rangeStrings(r(self)); ok {
				return object.Nil, nil
			}
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
		if _, ok := r(self).Lo.AsInt(); ok && len(args) == 0 && block.IsZero() {
			return c.Methods["size"].Native(ctx, self, nil, nil, object.Value{})
		}
		return object.Enumerable.Methods["count"].Native(ctx, self, args, kw, block)
	})
	cover := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in, err := rangeIncludes(ctx, r(self), args[0])
		return object.Bool(in), err
	}
	// include? walks a range of strings, as Ruby does: "bb" lies between
	// "a" and "z" but is not one of the strings from "a" to "z".
	include := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		seq, sym, endless, ok := rangeStrings(r(self))
		if !ok {
			return cover(ctx, self, args, kw, block)
		}
		if endless {
			return object.Value{}, ctx.Raise(object.TypeError, "cannot determine inclusion in beginless/endless ranges")
		}
		for s := range seq {
			if object.Equal(strValue(s, sym), args[0]) {
				return object.True, nil
			}
		}
		return object.False, nil
	}
	def(c, "include?", 1, include)
	def(c, "member?", 1, include)
	def(c, "cover?", 1, cover)
	def(c, "===", 1, cover)
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		if seq, sym, _, ok := rangeStrings(r(self)); ok {
			for s := range seq {
				if _, err := ctx.Yield(block, strValue(s, sym)); err != nil {
					return object.Value{}, err
				}
			}
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
		elems, err := RangeElems(ctx, r(self))
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
		elems, err := RangeElems(ctx, r(self))
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
	def(c, "bsearch", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "bsearch"); err != nil {
			return object.Value{}, err
		}
		return bsearch(ctx, r(self), block)
	})
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems, err := RangeElems(ctx, r(self))
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
	// min and max of a range compare its ends rather than walk it, but for
	// a block, a count, or a range without a last element to name.
	def(c, "min", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, hi := object.OrNil(r(self).Lo), object.OrNil(r(self).Hi)
		_, numeric := hi.AsNumber()
		if len(args) > 0 || !block.IsZero() || r(self).Exclusive && !numeric {
			return object.Enumerable.Methods["min"].Native(ctx, self, args, kw, block)
		}
		if lo.IsNil() {
			return object.Value{}, ctx.Raise(object.RangeError, "cannot get the minimum of beginless range")
		}
		if hi.IsNil() {
			return lo, nil
		}
		cmp, err := ctx.Compare(lo, hi)
		if err != nil {
			return object.Value{}, err
		}
		if cmp > 0 || cmp == 0 && r(self).Exclusive {
			return object.Nil, nil
		}
		return lo, nil
	})
	def(c, "max", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, hi := object.OrNil(r(self).Lo), object.OrNil(r(self).Hi)
		if hi.IsNil() {
			return object.Value{}, ctx.Raise(object.RangeError, "cannot get the maximum of endless range")
		}
		_, numeric := hi.AsNumber()
		if len(args) > 0 || !block.IsZero() || r(self).Exclusive && !numeric {
			return object.Enumerable.Methods["max"].Native(ctx, self, args, kw, block)
		}
		cmp := -1
		if !lo.IsNil() {
			var err error
			if cmp, err = ctx.Compare(lo, hi); err != nil {
				return object.Value{}, err
			}
		}
		if cmp > 0 {
			return object.Nil, nil
		}
		if !r(self).Exclusive {
			return hi, nil
		}
		hiI, ok := hi.AsInt()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "cannot exclude non Integer end value")
		}
		if cmp == 0 {
			return object.Nil, nil
		}
		if _, ok := lo.AsInt(); !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "cannot exclude end value with non Integer begin value")
		}
		return object.Int(hiI - 1), nil
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

// rangeStrings is the strings of a range over strings or symbols, whether
// they are symbols and whether the range is endless; ok is false for any
// other range.
func rangeStrings(r *object.Range) (seq iter.Seq[string], sym, endless, ok bool) {
	hi := object.OrNil(r.Hi)
	endless = hi.IsNil()
	if lo, isStr := r.Lo.AsString(); isStr {
		if h, isStr := hi.AsString(); isStr || endless {
			return stringsUpto(lo, h, r.Exclusive, endless), false, endless, true
		}
	}
	if lo, isSym := r.Lo.AsSymbol(); isSym {
		if h, isSym := hi.AsSymbol(); isSym || endless {
			return stringsUpto(lo, h, r.Exclusive, endless), true, endless, true
		}
	}
	return nil, false, false, false
}

// strValue is s as a string or, for a range of symbols, a symbol.
func strValue(s string, sym bool) object.Value {
	if sym {
		return object.Symbol(s)
	}
	return object.String(s)
}

// RangeElems expands a range of integers, strings or symbols into its
// elements.
func RangeElems(ctx object.Context, r *object.Range) ([]object.Value, error) {
	if seq, sym, endless, ok := rangeStrings(r); ok {
		if endless {
			return nil, ctx.Raise(object.RangeError, "cannot convert endless range to a list")
		}
		out := []object.Value{}
		for s := range seq {
			out = append(out, strValue(s, sym))
		}
		return out, nil
	}
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

// bsearch is Range#bsearch over integers or floats, after CRuby's
// range_bsearch. In find-minimum mode the block returns true or false and
// bsearch returns the smallest value it is true for; in find-any mode the
// block returns a number, 0 for a value it accepts, negative to look
// lower, positive to look higher. An endless or beginless integer range is
// probed at doubling distances from its end until the block turns.
func bsearch(ctx object.Context, r *object.Range, block object.Value) (object.Value, error) {
	satisfied := object.Nil
	// check yields v; found is a value accepted in find-any mode, smaller
	// says to look below v.
	check := func(v object.Value) (found, smaller bool, err error) {
		res, err := ctx.Yield(block, v)
		if err != nil {
			return false, false, err
		}
		switch {
		case res.Kind() == object.KindBool && res.Bool():
			satisfied = v
			return false, true, nil
		case !res.Truthy():
			return false, false, nil
		}
		f, ok := res.AsNumber()
		if !ok {
			return false, false, ctx.Raise(object.TypeError, "wrong argument type %s (must be numeric, true, false or nil)", kindName(res))
		}
		return f == 0, f < 0, nil
	}
	// search bisects low..high, high excluded when exclusive, over int64s
	// that conv turns into values.
	search := func(low, high int64, exclusive bool, conv func(int64) object.Value) (object.Value, error) {
		if !exclusive {
			high++
		}
		low--
		for low+1 < high {
			var mid int64
			if (high < 0) == (low < 0) {
				mid = low + (high-low)/2
			} else {
				mid = (low + high) / 2
			}
			found, smaller, err := check(conv(mid))
			if err != nil || found {
				return conv(mid), err
			}
			if smaller {
				high = mid
			} else {
				low = mid
			}
		}
		return satisfied, nil
	}
	lo, hi := object.OrNil(r.Lo), object.OrNil(r.Hi)
	_, loFloat := lo.AsFloat()
	_, hiFloat := hi.AsFloat()
	if loFloat || hiFloat {
		l, h := math.Inf(-1), math.Inf(1)
		if !lo.IsNil() {
			l, _ = lo.AsNumber()
		}
		if !hi.IsNil() {
			h, _ = hi.AsNumber()
		}
		return search(floatOrder(l), floatOrder(h), r.Exclusive, func(i int64) object.Value { return object.Float(orderFloat(i)) })
	}
	asInt := func(i int64) object.Value { return object.Int(i) }
	l, loInt := lo.AsInt()
	h, hiInt := hi.AsInt()
	switch {
	case loInt && hiInt:
		return search(l, h, r.Exclusive, asInt)
	case loInt && hi.IsNil():
		for diff := int64(1); diff > 0; diff *= 2 {
			found, smaller, err := check(object.Int(l + diff))
			if err != nil || found {
				return object.Int(l + diff), err
			}
			if smaller {
				return search(l, l+diff, false, asInt)
			}
		}
	case lo.IsNil() && hiInt:
		for diff := int64(-1); diff < 0; diff *= 2 {
			found, smaller, err := check(object.Int(h + diff))
			if err != nil || found {
				return object.Int(h + diff), err
			}
			if !smaller {
				return search(h+diff, h, r.Exclusive, asInt)
			}
		}
	default:
		return object.Value{}, ctx.Raise(object.TypeError, "can't do binary search for %s", kindName(lo))
	}
	return object.Value{}, ctx.Raise(object.RangeError, "bsearch went past the 64-bit integers")
}

// floatOrder maps a float to an int64 in the same order, as CRuby's
// double_as_int64 does, so that bisecting the integers bisects the floats;
// orderFloat maps it back.
func floatOrder(f float64) int64 {
	i := int64(math.Float64bits(math.Abs(f)))
	if f < 0 {
		return -i
	}
	return i
}

func orderFloat(i int64) float64 {
	if i < 0 {
		return -math.Float64frombits(uint64(-i))
	}
	return math.Float64frombits(uint64(i))
}
