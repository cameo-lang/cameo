package stdlib

import (
	"math"
	"math/big"

	"github.com/cameo-lang/cameo/src/object"
)

// An Enumerator::ArithmeticSequence is what Numeric#step and Range#step
// return without a block: an enumerator of begin, begin + step, ... up to
// its end, which knows its size and last element without counting. It
// counts in Integers when begin, step and end are Integers, and otherwise
// in Floats as CRuby's ruby_float_step does, each one begin + i * step so
// that no error accumulates.

// arithSeq makes an arithmetic sequence value; recv, meth, args and kw are
// the call that made it, for inspect.
func arithSeq(recv object.Value, meth string, args []object.Value, kw *object.Map, s *object.ArithSeq) object.Value {
	return object.EnumeratorValue(&object.Enumerator{
		Recv: recv, Meth: meth, Args: args, Kw: kw, Arith: s,
		Each: func(ctx object.Context, yield func(object.Value) error) error {
			return arithEach(ctx, s, yield)
		},
		Size: func(ctx object.Context) (object.Value, error) { return arithSize(s), nil },
	})
}

// arithInts reports a sequence that counts in Integers.
func arithInts(s *object.ArithSeq) bool {
	return s.Begin.IsInteger() && s.Step.IsInteger() && (s.End.IsInteger() || s.End.IsNil())
}

// arithFloats returns a sequence's begin, end and step as floats, the end
// an infinity for an endless one, and how many elements it has.
func arithFloats(s *object.ArithSeq) (beg, end, unit, n float64) {
	beg, _ = s.Begin.AsNumber()
	unit, _ = s.Step.AsNumber()
	end = math.Inf(1)
	if unit < 0 {
		end = math.Inf(-1)
	}
	if !s.End.IsNil() {
		end, _ = s.End.AsNumber()
	}
	return beg, end, unit, floatStepSize(beg, end, unit, s.Excl)
}

// arithFloat is the i-th element of a float sequence, never past its end.
func arithFloat(beg, end, unit, i float64) float64 {
	d := i*unit + beg
	if unit >= 0 && end < d || unit < 0 && d < end {
		d = end
	}
	return d
}

// arithEach yields the elements of a sequence.
func arithEach(ctx object.Context, s *object.ArithSeq, yield func(object.Value) error) error {
	if s.Begin.IsNil() {
		return ctx.Raise(object.ArgumentError, "#step iteration for beginless ranges is meaningless")
	}
	if !arithInts(s) {
		beg, end, unit, n := arithFloats(s)
		if math.IsInf(unit, 0) {
			// i * unit is NaN for i = 0, so Ruby yields beg alone.
			if n > 0 {
				return yield(object.Float(beg))
			}
			return nil
		}
		for i := float64(0); i < n; i++ {
			if err := yield(object.Float(arithFloat(beg, end, unit, i))); err != nil {
				return err
			}
		}
		return nil
	}
	from, fromInt := s.Begin.AsInt()
	to, toInt := s.End.AsInt()
	step, stepInt := s.Step.AsInt()
	if s.Excl && toInt {
		// The last element before an excluded end, for the loop below.
		switch {
		case step > 0 && to > math.MinInt64:
			to--
		case step < 0 && to < math.MaxInt64:
			to++
		default:
			toInt = false
		}
	}
	// In 64 bits while no step can pass the end of them.
	if fromInt && stepInt && toInt && (step > 0 && to <= math.MaxInt64-step || step < 0 && to >= math.MinInt64-step) {
		for i := from; step > 0 && i <= to || step < 0 && i >= to; i += step {
			if err := yield(object.Int(i)); err != nil {
				return err
			}
		}
		return nil
	}
	up := bigOf(s.Step).Sign() > 0
	for i := s.Begin; ; i = addIntegers(i, s.Step) {
		if !s.End.IsNil() {
			c, _ := object.CompareNumbers(i, s.End)
			if up && (c > 0 || s.Excl && c == 0) || !up && (c < 0 || s.Excl && c == 0) {
				return nil
			}
		}
		if err := yield(i); err != nil {
			return err
		}
	}
}

// arithCount is the number of elements of a finite Integer sequence.
func arithCount(s *object.ArithSeq) *big.Int {
	d := new(big.Int).Sub(bigOf(s.End), bigOf(s.Begin))
	step := bigOf(s.Step)
	if step.Sign() < 0 {
		d.Neg(d)
		step = new(big.Int).Neg(step)
	}
	if d.Sign() < 0 {
		return new(big.Int)
	}
	q, m := new(big.Int).QuoRem(d, step, new(big.Int))
	if !s.Excl || m.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

// arithSize is ArithmeticSequence#size: an Integer, or Infinity for an
// endless sequence.
func arithSize(s *object.ArithSeq) object.Value {
	if arithInts(s) {
		if s.End.IsNil() {
			return object.Float(math.Inf(1))
		}
		return object.BigInt(arithCount(s))
	}
	_, _, _, n := arithFloats(s)
	if math.IsInf(n, 1) {
		return object.Float(n)
	}
	return object.FloatToInteger(n)
}

// arithLast returns the last count elements of a finite sequence, in
// order.
func arithLast(ctx object.Context, s *object.ArithSeq, count int64) ([]object.Value, error) {
	if s.End.IsNil() {
		return nil, ctx.Raise(object.RangeError, "cannot get the last element of endless arithmetic sequence")
	}
	if arithInts(s) {
		size := arithCount(s)
		if size.Cmp(big.NewInt(count)) < 0 {
			count = size.Int64()
		}
		out := make([]object.Value, count)
		for k := int64(0); k < count; k++ {
			// begin + (size - count + k) * step
			i := new(big.Int).Sub(size, big.NewInt(count-k))
			i.Mul(i, bigOf(s.Step))
			out[k] = object.BigInt(i.Add(i, bigOf(s.Begin)))
		}
		return out, nil
	}
	// As Ruby's arith_seq_last: the last element is begin + step *
	// floor((end - begin) / step), one step back if that is an excluded
	// end, and the last count step forward to it from count - 1 steps
	// back.
	beg, _ := s.Begin.AsNumber()
	end, _ := s.End.AsNumber()
	unit, _ := s.Step.AsNumber()
	steps := math.Floor((end - beg) / unit)
	if math.IsInf(steps, 0) || math.IsNaN(steps) {
		return nil, ctx.Raise(object.RangeError, "cannot get the last element of endless arithmetic sequence")
	}
	if steps < 0 {
		return nil, nil
	}
	last, size := beg+unit*steps, steps+1
	if s.Excl && last == end {
		last, size = last-unit, steps
	}
	if float64(count) > size {
		count = int64(size)
	}
	out := make([]object.Value, count)
	x := last - float64(count-1)*unit
	for k := range out {
		out[k] = object.Float(x)
		x += unit
	}
	return out, nil
}

func installArithSeq() {
	c := object.ArithSeqClass
	seq := func(self object.Value) *object.ArithSeq { return self.Enumerator().Arith }
	reader := func(name string, f func(s *object.ArithSeq) object.Value) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(seq(self)), nil
		})
	}
	reader("begin", func(s *object.ArithSeq) object.Value { return s.Begin })
	reader("end", func(s *object.ArithSeq) object.Value { return s.End })
	reader("step", func(s *object.ArithSeq) object.Value { return s.Step })
	reader("exclude_end?", func(s *object.ArithSeq) object.Value { return object.Bool(s.Excl) })
	reader("size", arithSize)
	def(c, "last", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if len(args) == 0 {
			out, err := arithLast(ctx, seq(self), 1)
			if err != nil || len(out) == 0 {
				return object.Nil, err
			}
			return out[0], nil
		}
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative array size")
		}
		out, err := arithLast(ctx, seq(self), n)
		return object.NewList(out...), err
	})
	eq := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Equal(self, args[0])), nil
	}
	def(c, "==", 1, eq)
	def(c, "eql?", 1, eq)
	def(c, "===", 1, eq)
}

// arithInspect is ArithmeticSequence#inspect: the call that made it in
// parentheses, "(1.step(10, 3))" or "((1..10).step(3))".
func arithInspect(ctx object.Context, e *object.Enumerator) (string, error) {
	recv, err := ctx.Inspect(e.Recv)
	if err != nil {
		return "", err
	}
	if e.Recv.Kind() == object.KindRange {
		recv = "(" + recv + ")"
	}
	out := "(" + recv + "." + e.Meth
	var parts []string
	for _, a := range e.Args {
		s, err := ctx.Inspect(a)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	if e.Kw != nil {
		for i, k := range e.Kw.Keys() {
			s, err := ctx.Inspect(e.Kw.Values()[i])
			if err != nil {
				return "", err
			}
			name, _ := k.AsSymbol()
			parts = append(parts, name+": "+s)
		}
	}
	if len(parts) > 0 {
		out += "("
		for i, p := range parts {
			if i > 0 {
				out += ", "
			}
			out += p
		}
		out += ")"
	}
	return out + ")", nil
}

// combined makes an Enumerator::Chain or Enumerator::Product of parts.
func combined(of *object.Class, parts []object.Value) object.Value {
	e := &object.Enumerator{Of: of, Parts: parts}
	if of == object.ChainClass {
		e.Each = func(ctx object.Context, yield func(object.Value) error) error {
			for _, p := range parts {
				src, err := eachOf(ctx, p)
				if err != nil {
					return err
				}
				if err := src(ctx, yield); err != nil {
					return err
				}
			}
			return nil
		}
	} else {
		// The first part is the outermost loop; each inner part is
		// enumerated again for every element of the parts before it.
		var from func(ctx object.Context, i int, row []object.Value, yield func(object.Value) error) error
		from = func(ctx object.Context, i int, row []object.Value, yield func(object.Value) error) error {
			if i == len(parts) {
				return yield(object.NewList(append([]object.Value{}, row...)...))
			}
			src, err := eachOf(ctx, parts[i])
			if err != nil {
				return err
			}
			return src(ctx, func(v object.Value) error {
				return from(ctx, i+1, append(row, v), yield)
			})
		}
		e.Each = func(ctx object.Context, yield func(object.Value) error) error {
			return from(ctx, 0, nil, yield)
		}
	}
	e.Size = func(ctx object.Context) (object.Value, error) {
		return combinedSize(ctx, of == object.ChainClass, parts)
	}
	return object.EnumeratorValue(e)
}

// combinedSize is the sum (chain) or the product of the parts' sizes:
// nil when one is nil, a Float when one is.
func combinedSize(ctx object.Context, sum bool, parts []object.Value) (object.Value, error) {
	acc := object.Int(0)
	if !sum {
		acc = object.Int(1)
	}
	op := "+"
	if !sum {
		op = "*"
	}
	for _, p := range parts {
		n, err := ctx.Call(p, "size")
		if err != nil || n.IsNil() {
			return object.Nil, err
		}
		if acc, err = ctx.Call(acc, op, n); err != nil {
			return object.Value{}, err
		}
	}
	return acc, nil
}

// combinedInspect is "#<Enumerator::Chain: [part, ...]>".
func combinedInspect(ctx object.Context, e *object.Enumerator) (string, error) {
	parts, err := ctx.Inspect(object.NewList(e.Parts...))
	if err != nil {
		return "", err
	}
	return "#<" + e.Of.QualifiedName() + ": " + parts + ">", nil
}

func installCombined() {
	object.ChainClass.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combined(object.ChainClass, append([]object.Value{}, args...)), nil
	})
	newProduct := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p := combined(object.ProductClass, append([]object.Value{}, args...))
		if block.IsZero() {
			return p, nil
		}
		err := p.Enumerator().Each(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		return object.Nil, err
	}
	object.ProductClass.DefineSingleton("new", -1, newProduct)
	object.EnumeratorClass.DefineSingleton("product", -1, newProduct)
	def(object.Enumerable, "chain", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combined(object.ChainClass, append([]object.Value{self}, args...)), nil
	})
	def(object.EnumeratorClass, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combined(object.ChainClass, []object.Value{self, args[0]}), nil
	})
}
