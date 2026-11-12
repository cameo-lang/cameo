package stdlib

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// installEnumerable defines the Enumerable methods once, over the elements
// of whatever includes it: a List's elements, a Set's members, a Range's
// integers, a Map's [key, value] pairs.
func installEnumerable() {
	e := object.Enumerable
	each := func(name string, f func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error)) {
		def(e, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			elems, err := ToList(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, elems, args, block)
		})
	}
	// withBlock defines a method that takes a block, and without one
	// returns an enumerator, whose size is the receiver's when sized is set
	// and nil otherwise, as Ruby's does.
	withBlock := func(name string, sized bool, f func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error)) {
		def(e, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if block.IsZero() {
				return blockless(self, name, args, sized), nil
			}
			elems, err := ToList(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, elems, args, block)
		})
	}
	// compared is each for the methods that compare elements: it passes the
	// kinds they hold too.
	compared := func(name string, f func(ctx object.Context, elems []object.Value, kinds object.Kinds, args []object.Value, block object.Value) (object.Value, error)) {
		def(e, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			elems, err := ToList(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, elems, itemKinds(self, elems), args, block)
		})
	}
	// walked defines a method that calls its block with the receiver's
	// elements one after the other, as a stepper (walk.go), and without a
	// block returns an enumerator, as withBlock's methods do.
	walked := func(name string, sized bool, w walk) { e.DefineStep(name, -1, w.stepper(name, sized)) }
	walked("each_with_index", true, walk{
		yield: func(st []object.Value, el object.Value, i int, into []object.Value) []object.Value {
			return append(into, el, object.Int(i))
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return st[walkElems], nil
		},
	})
	// each_with_object keeps the memo: nothing mutates, so the block's
	// value is the next memo.
	withObject := walk{
		elems: func(ctx object.Context, call *object.Call) ([]object.Value, []object.Value, error) {
			elems, err := ToList(ctx, call.Self)
			return elems, []object.Value{call.Args[0]}, err
		},
		yield: func(st []object.Value, el object.Value, i int, into []object.Value) []object.Value {
			return append(into, el, st[walkKept])
		},
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			st[walkKept] = v
			return st, nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return st[walkKept], nil
		},
	}.stepper("each_with_object", true)
	e.DefineStep("each_with_object", -1, func(ctx object.Context, call *object.Call, st []object.Value, got object.Got) (object.Step, error) {
		if got.First {
			if len(call.Args) != 1 {
				return object.Step{}, ArgError(ctx, len(call.Args), "1")
			}
			if call.Block.IsZero() {
				return object.Done(withMemo(blockless(call.Self, "each_with_object", call.Args, true), call.Self, call.Args[0])), nil
			}
		}
		return withObject(ctx, call, st, got)
	})
	// each_slice and each_cons check their size before they return an
	// enumerator, as Ruby's do.
	sliced := func(name string, count func(size, n int64) int64, f func(ctx object.Context, elems []object.Value, n int, block object.Value) (object.Value, error)) {
		def(e, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n, err := sizeArg(ctx, args)
			if err != nil {
				return object.Value{}, err
			}
			if block.IsZero() {
				size := sizeOf(self)
				return enumFor(self, name, args, func(ctx object.Context) (object.Value, error) {
					s, err := size(ctx)
					if i, ok := s.AsInt(); ok && err == nil {
						return object.Int(count(i, int64(n))), nil
					}
					return s, err
				}), nil
			}
			elems, err := ToList(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			// With a block they return the receiver, as Ruby's do
			// since 3.1.
			if _, err := f(ctx, elems, n, block); err != nil {
				return object.Value{}, err
			}
			return self, nil
		})
	}
	sliced("each_slice", func(size, n int64) int64 { return (size + n - 1) / n }, func(ctx object.Context, elems []object.Value, n int, block object.Value) (object.Value, error) {
		var slices []object.Value
		for i := 0; i < len(elems); i += n {
			end := i + n
			if end > len(elems) {
				end = len(elems)
			}
			slices = append(slices, object.NewList(append([]object.Value{}, elems[i:end]...)...))
		}
		return yieldEach(ctx, slices, block)
	})
	sliced("each_cons", func(size, n int64) int64 { return max(size-n+1, 0) }, func(ctx object.Context, elems []object.Value, n int, block object.Value) (object.Value, error) {
		var windows []object.Value
		for i := 0; i+n <= len(elems); i++ {
			windows = append(windows, object.NewList(append([]object.Value{}, elems[i:i+n]...)...))
		}
		return yieldEach(ctx, windows, block)
	})
	// map gathers the block's values.
	mapping := walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			return append(st, v), nil, nil
		},
		done:  gathered,
		fills: true,
	}
	walked("map", true, mapping)
	walked("collect", true, mapping)
	// flat_map gathers the elements of a list the block returns, and any
	// other value itself.
	flatMap := walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			if l, ok := v.AsList(); ok {
				return append(st, l.Elems...), nil, nil
			}
			return append(st, v), nil, nil
		},
		done: gathered,
	}
	walked("flat_map", true, flatMap)
	walked("collect_concat", true, flatMap)
	walked("filter_map", true, walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			if v.Truthy() {
				st = append(st, v)
			}
			return st, nil, nil
		},
		done: gathered,
	})
	// select gathers the elements the block is true for, reject the
	// others.
	selecting := func(keep bool) walk {
		return walk{
			fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
				if v.Truthy() == keep {
					st = append(st, el)
				}
				return st, nil, nil
			},
			done: gathered,
		}
	}
	walked("select", true, selecting(true))
	walked("filter", true, selecting(true))
	walked("find_all", true, selecting(true))
	walked("reject", true, selecting(false))
	// partition keeps what the block said of each element, and sorts the
	// elements by it when the walk is through.
	walked("partition", true, walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			return append(st, object.Bool(v.Truthy())), nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			yes, no := []object.Value{}, []object.Value{}
			for i, el := range st[walkElems].List().Elems {
				if kept(st)[i].Truthy() {
					yes = append(yes, el)
				} else {
					no = append(no, el)
				}
			}
			return object.NewList(object.NewList(yes...), object.NewList(no...)), nil
		},
	})
	// find ends at the first element the block is true for.
	finding := walk{
		fold: func(ctx object.Context, st []object.Value, el, v object.Value) ([]object.Value, *object.Value, error) {
			if v.Truthy() {
				return st, &el, nil
			}
			return st, nil, nil
		},
		done: func(ctx object.Context, call *object.Call, st []object.Value) (object.Value, error) {
			return object.Nil, nil
		},
	}
	walked("find", false, finding)
	walked("detect", false, finding)
	def(e, "find_index", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() {
			return blockless(self, "find_index", nil, false), nil
		}
		elems, err := ToList(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		for i, el := range elems {
			hit, err := matches(ctx, el, args, block)
			if err != nil {
				return object.Value{}, err
			}
			if hit {
				return object.Int(i), nil
			}
		}
		return object.Nil, nil
	})
	each("inject", inject)
	each("reduce", inject)
	each("sum", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		var acc object.Value = object.Int(0)
		if len(args) == 1 {
			acc = args[0]
		}
		for _, el := range elems {
			v := el
			if !block.IsZero() {
				var err error
				if v, err = ctx.Yield(block, el); err != nil {
					return object.Value{}, err
				}
			}
			r, err := ctx.Call(acc, "+", v)
			if err != nil {
				return object.Value{}, err
			}
			acc = r
		}
		return acc, nil
	})
	each("count", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() {
			return object.Int(len(elems)), nil
		}
		n := 0
		for _, el := range elems {
			hit, err := matches(ctx, el, args, block)
			if err != nil {
				return object.Value{}, err
			}
			if hit {
				n++
			}
		}
		return object.Int(n), nil
	})
	extreme := func(name string, sign int) {
		compared(name, func(ctx object.Context, elems []object.Value, kinds object.Kinds, args []object.Value, block object.Value) (object.Value, error) {
			n := -1
			if len(args) == 1 {
				k, err := WantInt(ctx, args[0], "")
				if err != nil {
					return object.Value{}, err
				}
				n = int(k)
			}
			if n < 0 {
				return extremum(ctx, elems, kinds, block, sign)
			}
			sorted, err := sortValues(ctx, elems, kinds, block)
			if err != nil {
				return object.Value{}, err
			}
			if sign > 0 {
				reverse(sorted)
			}
			if n > len(sorted) {
				n = len(sorted)
			}
			return object.NewList(sorted[:n]...), nil
		})
	}
	extreme("min", -1)
	extreme("max", 1)
	extremeBy := func(name string, sign int) {
		withBlock(name, true, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			if len(args) == 0 {
				return extremumBy(ctx, elems, block, sign)
			}
			sorted, err := sortBy(ctx, elems, block)
			if err != nil {
				return object.Value{}, err
			}
			if sign > 0 {
				reverse(sorted)
			}
			if len(sorted) == 0 {
				return object.Nil, nil
			}
			if len(args) == 1 {
				n, err := WantInt(ctx, args[0], "")
				if err != nil {
					return object.Value{}, err
				}
				if int(n) > len(sorted) {
					n = int64(len(sorted))
				}
				return object.NewList(sorted[:n]...), nil
			}
			return sorted[0], nil
		})
	}
	extremeBy("min_by", -1)
	extremeBy("max_by", 1)
	// minmax_by calls the block once for each element, and keeps the first
	// of equal minimums and of equal maximums, as Ruby's does.
	withBlock("minmax_by", true, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if len(elems) == 0 {
			return object.NewList(object.Nil, object.Nil), nil
		}
		lo, hi := elems[0], elems[0]
		loKey, err := ctx.Yield(block, lo)
		if err != nil {
			return object.Value{}, err
		}
		hiKey := loKey
		for _, el := range elems[1:] {
			k, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if c, err := compareWith(ctx, object.Value{}, k, loKey); err != nil {
				return object.Value{}, err
			} else if c < 0 {
				lo, loKey = el, k
			}
			if c, err := compareWith(ctx, object.Value{}, k, hiKey); err != nil {
				return object.Value{}, err
			} else if c > 0 {
				hi, hiKey = el, k
			}
		}
		return object.NewList(lo, hi), nil
	})
	compared("minmax", func(ctx object.Context, elems []object.Value, kinds object.Kinds, args []object.Value, block object.Value) (object.Value, error) {
		lo, err := extremum(ctx, elems, kinds, block, -1)
		if err != nil {
			return object.Value{}, err
		}
		hi, err := extremum(ctx, elems, kinds, block, 1)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(lo, hi), nil
	})
	compared("sort", func(ctx object.Context, elems []object.Value, kinds object.Kinds, args []object.Value, block object.Value) (object.Value, error) {
		sorted, err := sortValues(ctx, elems, kinds, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(sorted...), nil
	})
	withBlock("sort_by", true, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		sorted, err := sortBy(ctx, elems, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(sorted...), nil
	})
	withBlock("group_by", true, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		groups := object.NewMapBuilder()
		var order []object.Value
		members := map[string][]object.Value{}
		for _, el := range elems {
			k, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			key := object.Key(k)
			if _, seen := members[key]; !seen {
				order = append(order, k)
			}
			members[key] = append(members[key], el)
		}
		for _, k := range order {
			groups.Set(k, object.NewList(members[object.Key(k)]...))
		}
		return object.MapValue(groups.Map()), nil
	})
	each("tally", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		counts := map[string]int64{}
		var order []object.Value
		for _, el := range elems {
			key := object.Key(el)
			if _, seen := counts[key]; !seen {
				order = append(order, el)
			}
			counts[key]++
		}
		b := object.NewMapBuilder()
		for _, el := range order {
			b.Set(el, object.Int(counts[object.Key(el)]))
		}
		return object.MapValue(b.Map()), nil
	})
	each("zip", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		others := make([][]object.Value, len(args))
		for i, a := range args {
			l, err := ToList(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			others[i] = l
		}
		out := make([]object.Value, len(elems))
		for i, el := range elems {
			tuple := []object.Value{el}
			for _, o := range others {
				tuple = append(tuple, at(o, i))
			}
			out[i] = object.NewList(tuple...)
		}
		if block.IsZero() {
			return object.NewList(out...), nil
		}
		// With a block, zip yields each tuple and returns nil.
		for _, t := range out {
			if _, err := ctx.Yield(block, t); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	each("take", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		n, err := countArg(ctx, args, len(elems))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems[:n]...)...), nil
	})
	each("drop", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		n, err := countArg(ctx, args, len(elems))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems[n:]...)...), nil
	})
	withBlock("take_while", false, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out := []object.Value{}
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if !v.Truthy() {
				break
			}
			out = append(out, el)
		}
		return object.NewList(out...), nil
	})
	withBlock("drop_while", false, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		i := 0
		for ; i < len(elems); i++ {
			v, err := ctx.Yield(block, elems[i])
			if err != nil {
				return object.Value{}, err
			}
			if !v.Truthy() {
				break
			}
		}
		return object.NewList(append([]object.Value{}, elems[i:]...)...), nil
	})
	each("first", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			if len(elems) == 0 {
				return object.Nil, nil
			}
			return elems[0], nil
		}
		n, err := countArg(ctx, args, len(elems))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems[:n]...)...), nil
	})
	include := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, ArgError(ctx, len(args), "1")
		}
		for _, el := range elems {
			eq, err := ctx.Equal(el, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if eq {
				return object.True, nil
			}
		}
		return object.False, nil
	}
	each("include?", include)
	each("member?", include)
	quantifier := func(name string, want func(hits, total int) bool) {
		each(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			hits := 0
			for _, el := range elems {
				hit, err := matches(ctx, el, args, block)
				if err != nil {
					return object.Value{}, err
				}
				if hit {
					hits++
				}
			}
			return object.Bool(want(hits, len(elems))), nil
		})
	}
	quantifier("any?", func(hits, total int) bool { return hits > 0 })
	quantifier("all?", func(hits, total int) bool { return hits == total })
	quantifier("none?", func(hits, total int) bool { return hits == 0 })
	quantifier("one?", func(hits, total int) bool { return hits == 1 })
	toA := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		return object.NewList(append([]object.Value{}, elems...)...), nil
	}
	each("to_a", toA)
	each("entries", toA)
	each("to_set", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if !block.IsZero() {
			mapped := make([]object.Value, len(elems))
			for i, el := range elems {
				v, err := ctx.Yield(block, el)
				if err != nil {
					return object.Value{}, err
				}
				mapped[i] = v
			}
			elems = mapped
		}
		return object.SetValue(object.NewSet(elems...)), nil
	})
	each("to_h", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for _, el := range elems {
			pair := el
			if !block.IsZero() {
				v, err := ctx.Yield(block, el)
				if err != nil {
					return object.Value{}, err
				}
				pair = v
			}
			l, ok := pair.AsList()
			if !ok || len(l.Elems) != 2 {
				return object.Value{}, ctx.Raise(object.TypeError, "wrong element type %s (expected a [key, value] pair)", KindName(pair))
			}
			b.Set(l.Elems[0], l.Elems[1])
		}
		return object.MapValue(b.Map()), nil
	})
	each("uniq", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		seen := map[string]bool{}
		out := []object.Value{}
		for _, el := range elems {
			k := el
			if !block.IsZero() {
				v, err := ctx.Yield(block, el)
				if err != nil {
					return object.Value{}, err
				}
				k = v
			}
			key := object.Key(k)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, el)
		}
		return object.NewList(out...), nil
	})
	// reverse_each returns its receiver, as each does.
	def(e, "reverse_each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return blockless(self, "reverse_each", nil, true), nil
		}
		elems, err := ToList(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		for i := len(elems) - 1; i >= 0; i-- {
			if _, err := ctx.Yield(block, elems[i]); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(e, "each_entry", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return blockless(self, "each_entry", args, true), nil
		}
		elems, err := ToList(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		if _, err := yieldEach(ctx, elems, block); err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	withBlock("chunk", true, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out := []object.Value{}
		err := chunker(ctx, block)(ctx, listEach(elems), func(v object.Value) error {
			out = append(out, v)
			return nil
		})
		return object.NewList(out...), err
	})
	// chunk_while, slice_when, slice_before and slice_after return the
	// slices as a list.
	for _, name := range []string{"chunk_while", "slice_when", "slice_before", "slice_after"} {
		each(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			slice, err := slicer(ctx, name, args, block)
			if err != nil {
				return object.Value{}, err
			}
			out := []object.Value{}
			err = slice(ctx, listEach(elems), func(v object.Value) error {
				out = append(out, v)
				return nil
			})
			return object.NewList(out...), err
		})
	}
	// cycle repeats the elements n times, or forever; without a block it
	// is an enumerator of them, of infinite size unless it is empty.
	def(e, "cycle", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := countArg(ctx, args, -1)
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			size := sizeOf(self)
			return enumFor(self, "cycle", args, func(ctx object.Context) (object.Value, error) {
				s, err := size(ctx)
				k, ok := s.AsInt()
				switch {
				case err != nil || !ok:
					return s, err
				case k == 0 || n == 0:
					return object.Int(0), nil
				case n < 0:
					return object.Float(math.Inf(1)), nil
				}
				return object.Int(k * int64(n)), nil
			}), nil
		}
		elems, err := ToList(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		cycle := func(ctx object.Context, yield func(object.Value) error) error {
			if len(elems) == 0 {
				return nil
			}
			for round := 0; n < 0 || round < n; round++ {
				for _, el := range elems {
					if err := yield(el); err != nil {
						return err
					}
				}
			}
			return nil
		}
		err = cycle(ctx, func(v object.Value) error {
			_, err := ctx.Yield(block, v)
			return err
		})
		return object.Nil, err
	})
	// grep keeps the elements pattern === matches, and grep_v the others;
	// a block maps what is kept.
	grep := func(name string, keep bool) {
		each(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			return grepElems(ctx, elems, args, block, keep)
		})
	}
	grep("grep", true)
	grep("grep_v", false)
	// lazy enumerates without listing first, so an endless range works.
	def(e, "lazy", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		each, err := eachOf(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		lazy := made(enumerator(true, self, "", nil, each), "lazy", object.Value{})
		// The lazy enumerator of an Array, Hash, Set or Range has its size.
		switch self.Kind() {
		case object.KindList, object.KindMap, object.KindSet, object.KindRange:
			lazy.Enumerator().Size = func(ctx object.Context) (object.Value, error) {
				return ctx.Call(self, "size")
			}
		}
		return lazy, nil
	})
}

// inject is reduce with a symbol, an initial value, a block, or both.
func inject(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
	var acc object.Value
	op := ""
	switch {
	case len(args) == 2:
		acc = args[0]
		s, err := WantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		op = s
	case len(args) == 1 && block.IsZero():
		s, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		op = s
	case len(args) == 1:
		acc = args[0]
	case block.IsZero():
		return object.Value{}, ctx.Raise(object.LocalJumpError, "no block given (inject)")
	}
	for _, el := range elems {
		if acc.IsZero() {
			acc = el
			continue
		}
		var err error
		if op != "" {
			acc, err = ctx.Call(acc, op, el)
		} else {
			acc, err = ctx.Yield(block, acc, el)
		}
		if err != nil {
			return object.Value{}, err
		}
	}
	if acc.IsZero() {
		return object.Nil, nil
	}
	return acc, nil
}

// matches tests an element against a pattern argument (through ===), a
// block, or truthiness when neither is given.
func matches(ctx object.Context, el object.Value, args []object.Value, block object.Value) (bool, error) {
	switch {
	case len(args) == 1:
		r, err := ctx.Call(args[0], "===", el)
		if err != nil {
			return false, err
		}
		return r.Truthy(), nil
	case !block.IsZero():
		r, err := ctx.Yield(block, el)
		if err != nil {
			return false, err
		}
		return r.Truthy(), nil
	}
	return el.Truthy(), nil
}

// yieldEach yields each element when there is a block, else returns the
// elements as a list.
func yieldEach(ctx object.Context, elems []object.Value, block object.Value) (object.Value, error) {
	if block.IsZero() {
		return object.NewList(elems...), nil
	}
	for _, el := range elems {
		if _, err := ctx.Yield(block, el); err != nil {
			return object.Value{}, err
		}
	}
	return object.Nil, nil
}

func sizeArg(ctx object.Context, args []object.Value) (int, error) {
	if len(args) != 1 {
		return 0, ArgError(ctx, len(args), "1")
	}
	n, err := WantInt(ctx, args[0], "")
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, ctx.Raise(object.ArgumentError, "invalid slice size")
	}
	return int(n), nil
}

// countArg reads an optional count, clamped to max when max is not -1.
func countArg(ctx object.Context, args []object.Value, max int) (int, error) {
	if len(args) == 0 {
		return max, nil
	}
	if len(args) != 1 {
		return 0, ArgError(ctx, len(args), "1")
	}
	n, err := WantInt(ctx, args[0], "")
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, ctx.Raise(object.ArgumentError, "attempt to take negative size")
	}
	if max >= 0 && int(n) > max {
		return max, nil
	}
	return int(n), nil
}

func at(elems []object.Value, i int) object.Value {
	if i < len(elems) {
		return elems[i]
	}
	return object.Nil
}

func reverse(elems []object.Value) {
	for i, j := 0, len(elems)-1; i < j; i, j = i+1, j-1 {
		elems[i], elems[j] = elems[j], elems[i]
	}
}

// sortValues sorts a copy of elems, which hold kinds, by <=>, or by a
// comparator block.
func sortValues(ctx object.Context, elems []object.Value, kinds object.Kinds, block object.Value) ([]object.Value, error) {
	out := append([]object.Value{}, elems...)
	if block.IsZero() {
		// slices sorts with typed swaps, where sort.SliceStable swaps by
		// reflection.
		if c := builtinCompare(kinds); c != nil {
			slices.SortStableFunc(out, c)
			return out, nil
		}
	}
	var sortErr error
	sort.SliceStable(out, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		var c int
		var err error
		if !block.IsZero() {
			r, err := ctx.Yield(block, out[i], out[j])
			if err != nil {
				sortErr = err
				return false
			}
			ci, ok := r.AsInt()
			if !ok {
				sortErr = ctx.Raise(object.ArgumentError, "comparison of %s with %s failed", KindName(out[i]), KindName(out[j]))
				return false
			}
			c = int(ci)
		} else if c, err = ctx.Compare(out[i], out[j]); err != nil {
			sortErr = err
			return false
		}
		return c < 0
	})
	if sortErr != nil {
		return nil, sortErr
	}
	return out, nil
}

// sortBy sorts a copy of elems by the keys a block computes.
func sortBy(ctx object.Context, elems []object.Value, block object.Value) ([]object.Value, error) {
	keys := make([]object.Value, len(elems))
	for i, el := range elems {
		k, err := ctx.Yield(block, el)
		if err != nil {
			return nil, err
		}
		keys[i] = k
	}
	return sortByKeys(ctx, elems, keys)
}

// sortByKeys returns elems in the order of their keys, key for element.
func sortByKeys(ctx object.Context, elems, keys []object.Value) ([]object.Value, error) {
	ints, nums := true, true
	for _, k := range keys {
		switch k.Kind() {
		case object.KindInt:
			// An Integer a float holds exactly, so that Integers and
			// Floats compare as floats.
			n := k.Int()
			nums = nums && n > -1<<53 && n < 1<<53
		case object.KindFloat:
			ints = false
			nums = nums && !math.IsNaN(k.Float())
		default:
			ints, nums = false, false
		}
	}
	// The elements are sorted by their key and then by their place, so
	// that elements with one key stay in their order whichever way the
	// sort goes about it. Keys that are numbers are sorted as numbers,
	// with the place beside them: no comparison calls back, and what
	// is moved is small.
	out := make([]object.Value, len(elems))
	switch {
	case ints:
		type keyed struct {
			key int64
			at  int
		}
		items := make([]keyed, len(keys))
		for i, k := range keys {
			items[i] = keyed{k.Int(), i}
		}
		slices.SortFunc(items, func(a, b keyed) int {
			if c := cmp.Compare(a.key, b.key); c != 0 {
				return c
			}
			return cmp.Compare(a.at, b.at)
		})
		for i, it := range items {
			out[i] = elems[it.at]
		}
	case nums:
		type keyed struct {
			key float64
			at  int
		}
		items := make([]keyed, len(keys))
		for i, k := range keys {
			if k.Kind() == object.KindInt {
				items[i] = keyed{float64(k.Int()), i}
			} else {
				items[i] = keyed{k.Float(), i}
			}
		}
		slices.SortFunc(items, func(a, b keyed) int {
			if c := cmp.Compare(a.key, b.key); c != 0 {
				return c
			}
			return cmp.Compare(a.at, b.at)
		})
		for i, it := range items {
			out[i] = elems[it.at]
		}
	default:
		at := make([]int, len(keys))
		for i := range at {
			at[i] = i
		}
		var sortErr error
		slices.SortFunc(at, func(a, b int) int {
			if sortErr != nil {
				return 0
			}
			c, err := ctx.Compare(keys[a], keys[b])
			if err != nil {
				sortErr = err
				return 0
			}
			if c != 0 {
				return c
			}
			return cmp.Compare(a, b)
		})
		if sortErr != nil {
			return nil, sortErr
		}
		for i, a := range at {
			out[i] = elems[a]
		}
	}
	return out, nil
}

func grepElems(ctx object.Context, elems []object.Value, args []object.Value, block object.Value, keep bool) (object.Value, error) {
	if len(args) != 1 {
		return object.Value{}, ArgError(ctx, len(args), "1")
	}
	out := []object.Value{}
	for _, el := range elems {
		r, err := ctx.Call(args[0], "===", el)
		if err != nil {
			return object.Value{}, err
		}
		if r.Truthy() == keep {
			if !block.IsZero() {
				if el, err = ctx.Yield(block, el); err != nil {
					return object.Value{}, err
				}
			}
			out = append(out, el)
		}
	}
	return object.NewList(out...), nil
}

// installUserEnumerable makes Enumerable's methods work for a user class
// that includes it and defines each: on such an object a method runs on an
// enumerator over its each, so that first, find, take_while and lazy stop
// as soon as they have their answer, even when each never ends.
func installUserEnumerable() {
	e := object.Enumerable
	names := make([]string, 0, len(e.Methods))
	for name, m := range e.Methods {
		if m.Native != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := e.Methods[name]
		native, step, name := m.Native, m.Step, name
		def(e, name, m.Arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if self.Kind() != object.KindInstance {
				return native(ctx, self, args, kw, block)
			}
			over := enumerator(false, self, "each", nil, func(ctx object.Context, yield func(object.Value) error) error {
				_, err := ctx.CallWith(self, "each", nil, nil, ctx.Block(func(args []object.Value) (object.Value, error) {
					return object.Nil, yield(yielded(args))
				}))
				return err
			})
			over.Enumerator().Size = sizeOf(self)
			r, err := ctx.CallWith(over, name, args, kw, block)
			// A blockless call returns an enumerator of over; it inspects
			// as one of the object, as in Ruby.
			if e, ok := r.AsEnumerator(); ok && err == nil && object.Same(e.Recv, over) {
				e.Recv = self
			}
			return r, err
		})
		// A method that is a stepper stays one for the receivers Go
		// defines; on a script's object it runs to its end in one step.
		if step != nil {
			wrapped := e.Methods[name]
			wrapped.Step = func(ctx object.Context, call *object.Call, st []object.Value, got object.Got) (object.Step, error) {
				if call.Self.Kind() != object.KindInstance {
					return step(ctx, call, st, got)
				}
				v, err := wrapped.Native(ctx, call.Self, call.Args, call.Kw, call.Block)
				return object.Done(v), err
			}
		}
	}
}

// itemKinds is the kinds of the items of an enumerable, self, whose items
// toList made elems: an Array and a Set keep them, anything else is
// scanned. It is for the Go implementations, which choose a path by it; a
// script has no use for it, and does not see it.
func itemKinds(self object.Value, elems []object.Value) object.Kinds {
	if l, ok := self.AsList(); ok {
		return l.Kinds()
	}
	if s, ok := self.AsSet(); ok {
		return s.Kinds()
	}
	return object.KindsOf(elems)
}

// builtinCompare is <=> for elements of kinds, when they are all Integers
// of the int64 form, all Floats other than NaN or all Strings, and nil
// otherwise. Those need no dispatch: no script can reopen a builtin class
// to redefine its <=>. A NaN compares with nothing, and is left to <=> to
// fail on.
func builtinCompare(kinds object.Kinds) func(a, b object.Value) int {
	switch {
	case kinds.Only(object.KindInt):
		return func(a, b object.Value) int { return cmp.Compare(a.Int(), b.Int()) }
	case kinds.Only(object.KindFloat):
		return func(a, b object.Value) int { return cmp.Compare(a.Float(), b.Float()) }
	case kinds.Only(object.KindString, object.KindBytes, object.KindASCII):
		return func(a, b object.Value) int { return strings.Compare(a.Str(), b.Str()) }
	}
	return nil
}

// compareWith compares a and b with the block, which returns their order
// as an Integer, or with <=> when there is no block.
func compareWith(ctx object.Context, block, a, b object.Value) (int, error) {
	if block.IsZero() {
		if a.Kind() == object.KindInt && b.Kind() == object.KindInt {
			return cmpInt(a.Int(), b.Int()), nil
		}
		return ctx.Compare(a, b)
	}
	r, err := ctx.Yield(block, a, b)
	if err != nil {
		return 0, err
	}
	c, ok := r.AsInt()
	if !ok {
		return 0, ctx.Raise(object.ArgumentError, "comparison of %s with %s failed", KindName(a), KindName(b))
	}
	return int(c), nil
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// extremum is the least element (sign -1) or the greatest (sign 1) in one
// pass, the first of equal ones as in Ruby, nil when there are none.
func extremum(ctx object.Context, elems []object.Value, kinds object.Kinds, block object.Value, sign int) (object.Value, error) {
	if len(elems) == 0 {
		return object.Nil, nil
	}
	best := elems[0]
	if c := builtinCompare(kinds); c != nil && block.IsZero() {
		for _, el := range elems[1:] {
			if c(el, best)*sign > 0 {
				best = el
			}
		}
		return best, nil
	}
	for _, el := range elems[1:] {
		c, err := compareWith(ctx, block, el, best)
		if err != nil {
			return object.Value{}, err
		}
		if c*sign > 0 {
			best = el
		}
	}
	return best, nil
}

// extremumBy is the element whose key the block computes is least (sign
// -1) or greatest (sign 1), in one pass, nil when there are none.
func extremumBy(ctx object.Context, elems []object.Value, block object.Value, sign int) (object.Value, error) {
	if len(elems) == 0 {
		return object.Nil, nil
	}
	best := elems[0]
	bestKey, err := ctx.Yield(block, best)
	if err != nil {
		return object.Value{}, err
	}
	for _, el := range elems[1:] {
		k, err := ctx.Yield(block, el)
		if err != nil {
			return object.Value{}, err
		}
		c, err := compareWith(ctx, object.Value{}, k, bestKey)
		if err != nil {
			return object.Value{}, err
		}
		if c*sign > 0 {
			best, bestKey = el, k
		}
	}
	return best, nil
}

// listEach produces the elements of a list.
func listEach(elems []object.Value) each {
	return func(ctx object.Context, yield func(object.Value) error) error {
		for _, el := range elems {
			if err := yield(el); err != nil {
				return err
			}
		}
		return nil
	}
}

// slicer reads the arguments of slice_before, slice_after, slice_when
// and chunk_while and returns how to cut a stream of elements: given the
// elements' source and where each finished slice goes.
func slicer(ctx object.Context, name string, args []object.Value, block object.Value) (func(ctx object.Context, src each, yield func(object.Value) error) error, error) {
	var cut func(prev, v object.Value) (bool, error)
	switch name {
	case "slice_before", "slice_after":
		switch {
		case len(args) == 0 && block.IsZero():
			return nil, ArgError(ctx, 0, "1")
		case len(args) > 0 && !block.IsZero():
			return nil, ArgError(ctx, len(args), "0")
		case len(args) > 1:
			return nil, ArgError(ctx, len(args), "1")
		}
		match := func(v object.Value) (bool, error) {
			var r object.Value
			var err error
			if block.IsZero() {
				r, err = ctx.Call(args[0], "===", v)
			} else {
				r, err = ctx.Yield(block, v)
			}
			return r.Truthy(), err
		}
		if name == "slice_after" {
			// A slice ends at a match, before the next element is read.
			return func(ctx object.Context, src each, yield func(object.Value) error) error {
				var cur []object.Value
				err := src(ctx, func(v object.Value) error {
					cur = append(cur, v)
					end, err := match(v)
					if err != nil || !end {
						return err
					}
					done := object.NewList(cur...)
					cur = nil
					return yield(done)
				})
				if err != nil || len(cur) == 0 {
					return err
				}
				return yield(object.NewList(cur...))
			}, nil
		}
		cut = func(prev, v object.Value) (bool, error) { return match(v) }
	default:
		if block.IsZero() {
			return nil, ctx.Raise(object.ArgumentError, "tried to create Proc object without a block")
		}
		if len(args) > 0 {
			return nil, ArgError(ctx, len(args), "0")
		}
		together := name == "chunk_while"
		cut = func(prev, v object.Value) (bool, error) {
			r, err := ctx.Yield(block, prev, v)
			return r.Truthy() != together, err
		}
	}
	return func(ctx object.Context, src each, yield func(object.Value) error) error {
		var cur []object.Value
		err := src(ctx, func(v object.Value) error {
			if len(cur) > 0 {
				c, err := cut(cur[len(cur)-1], v)
				if err != nil {
					return err
				}
				if c {
					done := object.NewList(cur...)
					cur = nil
					if err := yield(done); err != nil {
						return err
					}
				}
			} else if name == "slice_before" {
				// The first element starts a slice whatever it is, but
				// the block still sees it.
				if _, err := cut(object.Nil, v); err != nil {
					return err
				}
			}
			cur = append(cur, v)
			return nil
		})
		if err != nil || len(cur) == 0 {
			return err
		}
		return yield(object.NewList(cur...))
	}, nil
}

// chunker returns how Enumerable#chunk cuts a stream: runs of elements
// for which the block returns the same key, as [key, elements]. A nil or
// :_separator key drops the element and ends the run, :_alone puts it in
// a run of its own, and other keys that start with "_" are Ruby's.
func chunker(ctx object.Context, block object.Value) func(ctx object.Context, src each, yield func(object.Value) error) error {
	return func(ctx object.Context, src each, yield func(object.Value) error) error {
		var key object.Value
		var cur []object.Value
		flush := func() error {
			if len(cur) == 0 {
				return nil
			}
			run := object.NewList(key, object.NewList(cur...))
			cur = nil
			return yield(run)
		}
		err := src(ctx, func(v object.Value) error {
			k, err := ctx.Yield(block, v)
			if err != nil {
				return err
			}
			name, isSym := k.AsSymbol()
			switch {
			case k.IsNil() || isSym && name == "_separator":
				return flush()
			case isSym && name == "_alone":
				if err := flush(); err != nil {
					return err
				}
				key, cur = k, []object.Value{v}
				return flush()
			case isSym && strings.HasPrefix(name, "_"):
				return ctx.Raise(object.RuntimeError, "symbols beginning with an underscore are reserved")
			}
			if len(cur) > 0 {
				same, err := ctx.Equal(key, k)
				if err != nil {
					return err
				}
				if !same {
					if err := flush(); err != nil {
						return err
					}
				}
			}
			key = k
			cur = append(cur, v)
			return nil
		})
		if err != nil {
			return err
		}
		return flush()
	}
}
