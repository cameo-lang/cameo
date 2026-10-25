package stdlib

import (
	crand "crypto/rand"
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
)

// installRandom defines Random, Kernel#rand and Kernel#srand. A Random is
// a handle on a generator: each draw advances the generator, and every
// holder of the handle draws from the same one. Kernel#rand, Random.rand
// and the class's other methods draw from the interpreter's generator.
// The draws follow CRuby's random.c, so a seeded script prints CRuby's
// numbers; see doc/cameo-vs-ruby.md.
func installRandom() {
	c := object.RandomClass
	o := object.ObjectClass
	errno := object.ErrnoModule
	object.EDOMError.Lexical = errno
	errno.Consts["EDOM"] = object.ClassValue(object.EDOMError)

	def(o, "rand", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		return kernelRand(ctx, args)
	})
	srand := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		seed := object.NewSeed()
		if len(args) == 1 {
			var err error
			if seed, err = toInt(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return ctx.Rand().Reseed(seed), nil
	}
	def(o, "srand", -1, srand)
	c.DefineSingleton("srand", -1, srand)

	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		seed := object.NewSeed()
		if len(args) == 1 {
			var err error
			if seed, err = toInt(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return object.RandomValue(object.NewRandom(seed)), nil
	})
	c.DefineSingleton("new_seed", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewSeed(), nil
	})
	c.DefineSingleton("seed", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Rand().Seed(), nil
	})
	c.DefineSingleton("rand", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return randomRand(ctx, ctx.Rand(), args)
	})
	c.DefineSingleton("random_number", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return randomNumber(ctx, ctx.Rand(), args)
	})
	c.DefineSingleton("bytes", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := byteCount(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(ctx.Rand().Bytes(n)), nil
	})
	c.DefineSingleton("urandom", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := byteCount(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		b := make([]byte, n)
		_, _ = crand.Read(b)
		return object.Bytes(b), nil
	})

	def(c, "rand", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return randomRand(ctx, self.Random(), args)
	})
	def(c, "random_number", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return randomNumber(ctx, self.Random(), args)
	})
	def(c, "bytes", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := byteCount(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bytes(self.Random().Bytes(n)), nil
	})
	def(c, "seed", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.Random().Seed(), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsRandom()
		return object.Bool(ok && self.Random().SameState(o)), nil
	})
	// A copy would draw the numbers its original draws next.
	noCopy := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.TypeError, "a Random cannot be copied")
	}
	def(c, "dup", -1, noCopy)
	def(c, "clone", -1, noCopy)
}

// kernelRand is Kernel#rand, rb_f_rand of CRuby: a float in [0, 1)
// without an argument or with nil or zero, an integer below max.to_i.abs
// for a number, and a number within a range, or nil for an empty one.
func kernelRand(ctx object.Context, args []object.Value) (object.Value, error) {
	r := ctx.Rand()
	if len(args) == 1 && !args[0].IsNil() {
		v, isRange, err := randRange(ctx, r, args[0])
		if err != nil || isRange {
			return v, err
		}
		max, err := toInt(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if i, ok := max.AsInt(); !ok || i != 0 {
			if v := randInt(r, max, false); !v.IsNil() {
				return v, nil
			}
		}
	}
	return object.Float(r.Float(false)), nil
}

// randomRand is Random#rand and Random.rand, rand_random and
// check_random_number of CRuby: like Kernel#rand, but a negative or zero
// limit or an empty range is an ArgumentError, and a Float limit is not
// truncated.
func randomRand(ctx object.Context, r *object.Random, args []object.Value) (object.Value, error) {
	v, ok, err := randomDraw(ctx, r, args)
	if err != nil {
		return object.Value{}, err
	}
	if !ok {
		if _, err := wantInt(ctx, args[0], ""); err != nil {
			return object.Value{}, err
		}
	}
	if v.IsNil() {
		return object.Value{}, invalidArgument(ctx, args[0])
	}
	return v, nil
}

// randomNumber is Random#random_number: Random#rand, with a float in
// [0, 1) where rand fails for nil, zero or a negative limit.
func randomNumber(ctx object.Context, r *object.Random, args []object.Value) (object.Value, error) {
	v, ok, err := randomDraw(ctx, r, args)
	if err != nil {
		return object.Value{}, err
	}
	if !ok {
		return object.Value{}, invalidArgument(ctx, args[0])
	}
	if v.IsNil() {
		return object.Float(r.Float(false)), nil
	}
	return v, nil
}

// randomDraw is rand_random of CRuby: the draw for Random#rand's
// argument, nil for a limit it refuses, and ok false for an argument that
// is neither a number nor a range of numbers.
func randomDraw(ctx object.Context, r *object.Random, args []object.Value) (v object.Value, ok bool, err error) {
	if len(args) > 1 {
		return object.Value{}, true, argError(ctx, len(args), "0..1")
	}
	if len(args) == 0 {
		return object.Float(r.Float(false)), true, nil
	}
	max := args[0]
	if max.IsNil() {
		return object.Nil, true, nil
	}
	if max.Kind() != object.KindFloat {
		if i, ok, err := checkInt(ctx, max); err != nil || ok {
			return randInt(r, i, true), true, err
		}
	}
	if f, ok := checkFloat(max); ok {
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return object.Value{}, true, domainError(ctx)
		}
		if f < 0 {
			return object.Nil, true, nil
		}
		x := r.Float(false)
		if f > 0 {
			x *= f
		}
		return object.Float(x), true, nil
	}
	v, isRange, err := randRange(ctx, r, max)
	return v, isRange, err
}

// randInt is rand_int of CRuby: an integer in [0, max), nil for a zero
// max, and for a negative one nil when restrictive and an integer below
// its magnitude otherwise.
func randInt(r *object.Random, max object.Value, restrictive bool) object.Value {
	if i, ok := max.AsInt(); ok && i != math.MinInt64 {
		if i == 0 || i < 0 && restrictive {
			return object.Nil
		}
		if i < 0 {
			i = -i
		}
		return object.Int(int64(r.BelowInt(uint64(i) - 1)))
	}
	b, _ := max.AsBigInt()
	if b.Sign() < 0 && restrictive {
		return object.Nil
	}
	limit := new(big.Int).Abs(b)
	return object.BigInt(r.Below(limit.Sub(limit, big.NewInt(1))))
}

// randRange is rand_range of CRuby: a number within the range vmax, nil
// for an empty one, and isRange false when vmax is not a range whose end
// answers -. An endless or beginless range is outside the domain.
func randRange(ctx object.Context, r *object.Random, vmax object.Value) (v object.Value, isRange bool, err error) {
	rg, ok := vmax.AsRange()
	if !ok {
		return object.Value{}, false, nil
	}
	beg, end := object.OrNil(rg.Lo), object.OrNil(rg.Hi)
	if beg.IsNil() || end.IsNil() {
		return object.Value{}, true, domainError(ctx)
	}
	if !ctx.RespondsTo(end, "-") {
		return object.Value{}, false, nil
	}
	span, err := ctx.Call(end, "-", beg)
	if err != nil {
		return object.Value{}, true, err
	}
	v = span
	if span.Kind() != object.KindFloat {
		if n, ok, err := checkInt(ctx, span); err != nil {
			return object.Value{}, true, err
		} else if ok {
			v = object.Nil
			limit, _ := n.AsBigInt()
			if rg.Exclusive {
				limit = new(big.Int).Sub(limit, big.NewInt(1))
			}
			if limit.Sign() >= 0 {
				if limit.IsUint64() {
					v = object.BigInt(new(big.Int).SetUint64(r.BelowInt(limit.Uint64())))
				} else {
					v = object.BigInt(r.Below(limit))
				}
			}
			return addBegin(ctx, beg, v)
		}
	}
	if max, ok := checkFloat(span); ok {
		v = object.Nil
		scale, mid := 1.0, 0.5
		if math.IsInf(max, 0) {
			lo, err := finiteFloat(ctx, beg)
			if err != nil {
				return object.Value{}, true, err
			}
			hi, err := finiteFloat(ctx, end)
			if err != nil {
				return object.Value{}, true, err
			}
			lo, max = lo/2, hi/2
			scale, mid = 2, max+lo
			max -= lo
		} else if math.IsNaN(max) {
			return object.Value{}, true, domainError(ctx)
		}
		switch {
		case max > 0:
			x := r.Float(!rg.Exclusive)
			if scale > 1 {
				return object.Float((x-0.5)*max*scale + mid), true, nil
			}
			v = object.Float(x * max)
		case max == 0 && !rg.Exclusive:
			v = object.Float(0)
		}
	}
	return addBegin(ctx, beg, v)
}

// addBegin returns beg + v, the end of rand_range: nil stays nil.
func addBegin(ctx object.Context, beg, v object.Value) (object.Value, bool, error) {
	switch {
	case v.IsNil():
		return object.Nil, true, nil
	case v.IsInteger() && beg.IsInteger():
		x, _ := beg.AsBigInt()
		y, _ := v.AsBigInt()
		return object.BigInt(new(big.Int).Add(x, y)), true, nil
	case v.Kind() == object.KindFloat:
		if f, ok := checkFloat(beg); ok {
			return object.Float(v.Float() + f), true, nil
		}
	}
	sum, err := ctx.Call(beg, "+", v)
	return sum, true, err
}

// checkInt is rb_check_to_int for a value that is not a Float: an
// Integer, or what its to_int returns, and false for a value without one.
func checkInt(ctx object.Context, v object.Value) (object.Value, bool, error) {
	if v.IsInteger() {
		return v, true, nil
	}
	if v.Kind() == object.KindFloat || v.IsNil() || !ctx.RespondsTo(v, "to_int") {
		return object.Value{}, false, nil
	}
	i, err := ctx.Call(v, "to_int")
	if err != nil {
		return object.Value{}, true, err
	}
	return i, i.IsInteger(), nil
}

// checkFloat reads a number as a float, and false for anything else.
func checkFloat(v object.Value) (float64, bool) {
	if v.Kind() == object.KindFloat || v.IsInteger() {
		return v.AsNumber()
	}
	return 0, false
}

// finiteFloat is float_value of CRuby: a number as a finite float, or
// outside the domain.
func finiteFloat(ctx object.Context, v object.Value) (float64, error) {
	f, ok := checkFloat(v)
	if !ok {
		return 0, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(v))
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, domainError(ctx)
	}
	return f, nil
}

// toInt is rb_to_int: an Integer, a Float truncated, or what to_int
// returns, and a TypeError for anything else.
func toInt(ctx object.Context, v object.Value) (object.Value, error) {
	switch {
	case v.IsInteger():
		return v, nil
	case v.Kind() == object.KindFloat:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return object.Value{}, ctx.Raise(object.FloatDomainError, "%s", object.FormatFloat(f))
		}
		return object.FloatToInteger(f), nil
	case !v.IsNil() && v.Kind() != object.KindBool && ctx.RespondsTo(v, "to_int"):
		i, err := ctx.Call(v, "to_int")
		if err != nil {
			return object.Value{}, err
		}
		if i.IsInteger() {
			return i, nil
		}
	}
	return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(v))
}

// byteCount reads the length bytes takes.
func byteCount(ctx object.Context, v object.Value) (int, error) {
	n, err := wantInt(ctx, v, "")
	if err != nil {
		return 0, err
	}
	if n < 0 || n > math.MaxInt32 {
		return 0, ctx.Raise(object.ArgumentError, "negative string size (or size too big)")
	}
	return int(n), nil
}

func invalidArgument(ctx object.Context, v object.Value) error {
	s, err := ctx.ToS(v)
	if err != nil {
		return err
	}
	return ctx.Raise(object.ArgumentError, "invalid argument - %s", s)
}

func domainError(ctx object.Context) error {
	return ctx.Raise(object.EDOMError, "Numerical argument out of domain")
}

// randomOption reads the random: keyword of shuffle and sample: the
// generator to draw from, the interpreter's by default and for the class
// Random itself.
func randomOption(ctx object.Context, kw *object.Map) (object.Value, error) {
	gen := object.RandomValue(ctx.Rand())
	if kw == nil || kw.Len() == 0 {
		return gen, nil
	}
	for _, k := range kw.Keys() {
		if s, ok := k.AsSymbol(); !ok || s != "random" {
			text, err := ctx.Inspect(k)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", text)
		}
	}
	v, _ := kw.Get(object.Symbol("random"))
	if c, ok := v.AsClass(); ok && c == object.RandomClass {
		return gen, nil
	}
	return v, nil
}

// randUpto is RAND_UPTO of CRuby's array.c, rb_random_ulong_limited of
// max-1: an integer in [0, max) from gen, a Random or any object whose
// rand(max) returns one.
func randUpto(ctx object.Context, gen object.Value, max int) (int, error) {
	if r, ok := gen.AsRandom(); ok {
		return int(r.BelowInt(uint64(max - 1))), nil
	}
	v, err := ctx.Call(gen, "rand", object.Int(max))
	if err != nil {
		return 0, err
	}
	i, err := toInt(ctx, v)
	if err != nil {
		return 0, err
	}
	n, ok := i.AsInt()
	switch {
	case !ok || n >= int64(max):
		s, _ := ctx.Inspect(i)
		return 0, ctx.Raise(object.RangeError, "random number too big %s", s)
	case n < 0:
		return 0, ctx.Raise(object.RangeError, "random number too small %d", n)
	}
	return int(n), nil
}

// shuffle is rb_ary_shuffle of CRuby: from the end, each element swapped
// with one at or before it.
func shuffle(ctx object.Context, es []object.Value, gen object.Value) ([]object.Value, error) {
	out := append([]object.Value{}, es...)
	for i := len(out); i > 0; i-- {
		j, err := randUpto(ctx, gen, i)
		if err != nil {
			return nil, err
		}
		out[i-1], out[j] = out[j], out[i-1]
	}
	return out, nil
}

// sample is ary_sample of CRuby with a count: up to ten elements chosen
// from draws made first, more by the first n steps of a Fisher-Yates
// shuffle, with the swapped positions kept in a map so that a small
// sample of a large array costs n, not the array's length.
func sample(ctx object.Context, es []object.Value, n int, gen object.Value) ([]object.Value, error) {
	n = min(n, len(es))
	up := func(max int) (int, error) { return randUpto(ctx, gen, max) }
	if n <= 10 {
		rnds := make([]int, n)
		for i := range rnds {
			var err error
			if rnds[i], err = up(len(es) - i); err != nil {
				return nil, err
			}
		}
		switch n {
		case 0:
			return []object.Value{}, nil
		case 1:
			return []object.Value{es[rnds[0]]}, nil
		case 2:
			i, j := rnds[0], rnds[1]
			if j >= i {
				j++
			}
			return []object.Value{es[i], es[j]}, nil
		case 3:
			i, j, k := rnds[0], rnds[1], rnds[2]
			l, g := j, i
			if j >= i {
				l = i
				j++
				g = j
			}
			if k >= l {
				k++
				if k >= g {
					k++
				}
			}
			return []object.Value{es[i], es[j], es[k]}, nil
		}
		sorted := make([]int, 0, n)
		out := make([]object.Value, n)
		for i, k := range rnds {
			j := 0
			for ; j < len(sorted); j++ {
				if k < sorted[j] {
					break
				}
				k++
			}
			sorted = append(sorted, 0)
			copy(sorted[j+1:], sorted[j:])
			sorted[j] = k
			out[i] = es[k]
		}
		return out, nil
	}
	swapped := make(map[int]int, n)
	at := func(i int) int {
		if j, ok := swapped[i]; ok {
			return j
		}
		return i
	}
	out := make([]object.Value, n)
	for i := 0; i < n; i++ {
		r, err := up(len(es) - i)
		if err != nil {
			return nil, err
		}
		j := i + r
		vi, vj := at(i), at(j)
		swapped[j] = vi
		out[i] = es[vj]
	}
	return out, nil
}
