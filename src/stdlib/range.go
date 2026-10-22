package stdlib

import (
	"iter"
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
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
			if lo := r(self).Lo; lo.IsInteger() {
				for i := int64(0); i < n; i++ {
					out = append(out, object.BigInt(new(big.Int).Add(bigOf(lo), big.NewInt(i))))
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
		if r(self).Hi.IsZero() {
			return object.Value{}, ctx.Raise(object.RangeError, "cannot get the last element of endless range")
		}
		if len(args) == 1 {
			if n, ok := args[0].AsInt(); ok && n < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative array size")
			}
		}
		// The last n of a range of 64-bit integers count back from its
		// end, without making the elements before them.
		if lo, hi, ok := intEnds(r(self)); ok && len(args) == 1 {
			lo, okLo := lo.AsInt()
			hi, okHi := hi.AsInt()
			if okLo && okHi {
				// The size, which only bounds n, saturates where hi - lo + 1
				// leaves int.
				size := 0
				if hi >= lo {
					size = math.MaxInt
					if d := uint64(hi - lo); d < math.MaxInt {
						size = int(d) + 1
					}
				}
				n, err := countArg(ctx, args, size)
				if err != nil {
					return object.Value{}, err
				}
				out := make([]object.Value, n)
				for i := range out {
					out[i] = object.Int(hi - int64(n-1-i))
				}
				return object.NewList(out...), nil
			}
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
		if !r(self).Lo.IsInteger() {
			if _, _, _, ok := rangeStrings(r(self)); ok {
				return object.Nil, nil
			}
			return object.Value{}, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r(self).Lo)))
		}
		lo, hi, ok := intEnds(r(self))
		if !ok {
			if r(self).Hi.IsZero() {
				return object.Float(math.Inf(1)), nil
			}
			return object.Nil, nil
		}
		n := new(big.Int).Sub(bigOf(hi), bigOf(lo))
		if n.Add(n, big.NewInt(1)).Sign() < 0 {
			return object.Int(0), nil
		}
		return object.BigInt(n), nil
	})
	def(c, "count", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if r(self).Lo.IsInteger() && len(args) == 0 && block.IsZero() {
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
	def(c, "overlap?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].Kind() != object.KindRange {
			return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Range)", args[0].Class().Name)
		}
		return object.Bool(rangesOverlap(ctx, r(self), args[0].Range())), nil
	})
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
		if !r(self).Lo.IsInteger() {
			return object.Value{}, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r(self).Lo)))
		}
		if r(self).Hi.IsZero() {
			for i := r(self).Lo; ; {
				if _, err := ctx.Yield(block, i); err != nil {
					return object.Value{}, err
				}
				if x, ok := i.AsInt(); ok && x < math.MaxInt64 {
					i = object.Int(x + 1)
				} else {
					i = object.BigInt(new(big.Int).Add(bigOf(i), big.NewInt(1)))
				}
			}
		}
		lo, hi, ok := intEnds(r(self))
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r(self).Lo)))
		}
		err := eachInteger(lo, hi, 1, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		if err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	// reverse_each walks a range of integers down from its end, without
	// listing it first; any other range reverses its list.
	def(c, "reverse_each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, hi, ok := intEnds(r(self))
		if !ok || block.IsZero() {
			return object.Enumerable.Methods["reverse_each"].Native(ctx, self, args, kw, block)
		}
		err := eachInteger(hi, lo, -1, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		if err != nil {
			return object.Value{}, err
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
		if lo, hi, ok := intEnds(r(self)); ok && block.IsZero() && len(args) == 0 {
			l, h := bigOf(lo), bigOf(hi)
			if h.Cmp(l) < 0 {
				return object.Int(0), nil
			}
			// (lo+hi) * (hi-lo+1) / 2, which one of the factors makes even.
			n := new(big.Int).Sub(h, l)
			n.Add(n, big.NewInt(1))
			sum := new(big.Int).Add(l, h)
			return object.BigInt(sum.Rsh(sum.Mul(sum, n), 1)), nil
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
		if !hi.IsInteger() {
			return object.Value{}, ctx.Raise(object.TypeError, "cannot exclude non Integer end value")
		}
		if cmp == 0 {
			return object.Nil, nil
		}
		if !lo.IsInteger() {
			return object.Value{}, ctx.Raise(object.TypeError, "cannot exclude end value with non Integer begin value")
		}
		_, last, _ := intEnds(r(self))
		return last, nil
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
	lo, hi, ok := intEnds(r)
	if !ok {
		if r.Hi.IsZero() && r.Lo.IsInteger() {
			return nil, ctx.Raise(object.RangeError, "cannot convert endless range to a list")
		}
		return nil, ctx.Raise(object.TypeError, "can't iterate from %s", kindName(object.OrNil(r.Lo)))
	}
	out := []object.Value{}
	if lo, ok := lo.AsInt(); ok {
		if hi, ok := hi.AsInt(); ok {
			for i := lo; i <= hi; i++ {
				out = append(out, object.Int(i))
			}
			return out, nil
		}
	}
	err := eachInteger(lo, hi, 1, func(v object.Value) error {
		out = append(out, v)
		return nil
	})
	return out, err
}

// intEnds reads a range of integers as its first and last, the end less
// one for an exclusive range; ok is false for any other range.
func intEnds(r *object.Range) (lo, hi object.Value, ok bool) {
	if !r.Lo.IsInteger() || !r.Hi.IsInteger() {
		return object.Value{}, object.Value{}, false
	}
	hi = r.Hi
	if r.Exclusive {
		if h, ok := hi.AsInt(); ok && h > math.MinInt64 {
			hi = object.Int(h - 1)
		} else {
			hi = object.BigInt(new(big.Int).Sub(bigOf(hi), big.NewInt(1)))
		}
	}
	return r.Lo, hi, true
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
	// searchBig is CRuby's bsearch_integer_range, for bounds beyond 64 bits.
	searchBig := func(low, high *big.Int, exclusive bool) (object.Value, error) {
		low, high = new(big.Int).Sub(low, big.NewInt(1)), new(big.Int).Set(high)
		if !exclusive {
			high.Add(high, big.NewInt(1))
		}
		next := new(big.Int)
		for next.Add(low, big.NewInt(1)).Cmp(high) < 0 {
			mid := new(big.Int).Rsh(new(big.Int).Add(low, high), 1)
			v := object.BigInt(mid)
			found, smaller, err := check(v)
			if err != nil || found {
				return v, err
			}
			if smaller {
				high = mid
			} else {
				low = mid
			}
		}
		return satisfied, nil
	}
	// between searches low..high, in 64 bits where both fit.
	between := func(low, high *big.Int, exclusive bool) (object.Value, error) {
		if low.IsInt64() && high.IsInt64() {
			return search(low.Int64(), high.Int64(), exclusive, asInt)
		}
		return searchBig(low, high, exclusive)
	}
	// probe looks at from+diff for doubling diffs, toward the end a range
	// lacks, and searches between from and the first value that turns.
	probe := func(from *big.Int, up bool) (object.Value, error) {
		diff := big.NewInt(1)
		if !up {
			diff.Neg(diff)
		}
		for {
			at := new(big.Int).Add(from, diff)
			v := object.BigInt(at)
			found, smaller, err := check(v)
			if err != nil || found {
				return v, err
			}
			if smaller == up {
				if up {
					return between(from, at, false)
				}
				return between(at, from, r.Exclusive)
			}
			diff.Lsh(diff, 1)
		}
	}
	switch {
	case lo.IsInteger() && hi.IsInteger():
		return between(bigOf(lo), bigOf(hi), r.Exclusive)
	case lo.IsInteger() && hi.IsNil():
		return probe(bigOf(lo), true)
	case lo.IsNil() && hi.IsInteger():
		return probe(bigOf(hi), false)
	}
	return object.Value{}, ctx.Raise(object.TypeError, "can't do binary search for %s", kindName(lo))
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

// rangesOverlap follows Ruby's Range#overlap?: no value lies in both when
// either range ends before the other begins, or either is empty. Values
// that do not compare do not overlap.
func rangesOverlap(ctx object.Context, a, b *object.Range) bool {
	aLo, aHi, bLo, bHi := object.OrNil(a.Lo), object.OrNil(a.Hi), object.OrNil(b.Lo), object.OrNil(b.Hi)
	// empty is whether nothing lies from lo to hi.
	empty := func(lo, hi object.Value, exclusive bool) bool {
		if lo.IsNil() || hi.IsNil() {
			return false
		}
		c, err := ctx.Compare(lo, hi)
		return err != nil || c > 0 || c == 0 && exclusive
	}
	if empty(aLo, bHi, b.Exclusive) || empty(bLo, aHi, a.Exclusive) {
		return false
	}
	switch {
	case !aLo.IsNil() && !bLo.IsNil():
		c, err := ctx.Compare(aLo, bLo)
		if err != nil {
			return false
		}
		if c == 0 {
			return true
		}
	case aLo.IsNil() && bLo.IsNil():
		if aHi.IsNil() || bHi.IsNil() {
			return true
		}
		_, err := ctx.Compare(aHi, bHi)
		return err == nil
	}
	return !empty(aLo, aHi, a.Exclusive) && !empty(bLo, bHi, b.Exclusive)
}
