package stdlib

import (
	"sort"

	"github.com/radiospiel/amber/src/object"
)

// installEnumerable defines the Enumerable methods once, over the elements
// of whatever includes it: a List's elements, a Set's members, a Range's
// integers, a Map's [key, value] pairs.
func installEnumerable() {
	e := object.Enumerable
	each := func(name string, f func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error)) {
		def(e, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			elems, err := toList(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return f(ctx, elems, args, block)
		})
	}
	withBlock := func(name string, f func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error)) {
		each(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			if err := needBlock(ctx, block, name); err != nil {
				return object.Value{}, err
			}
			return f(ctx, elems, args, block)
		})
	}
	withBlock("each_with_index", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		for i, el := range elems {
			if _, err := ctx.Yield(block, el, object.Int(i)); err != nil {
				return object.Value{}, err
			}
		}
		return object.NewList(elems...), nil
	})
	withBlock("each_with_object", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, argError(ctx, len(args), "1")
		}
		memo := args[0]
		for _, el := range elems {
			v, err := ctx.Yield(block, el, memo)
			if err != nil {
				return object.Value{}, err
			}
			// Nothing mutates, so the block's value is the next memo.
			memo = v
		}
		return memo, nil
	})
	each("each_slice", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		n, err := sizeArg(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
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
	each("each_cons", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		n, err := sizeArg(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		var windows []object.Value
		for i := 0; i+n <= len(elems); i++ {
			windows = append(windows, object.NewList(append([]object.Value{}, elems[i:i+n]...)...))
		}
		return yieldEach(ctx, windows, block)
	})
	mapped := func(ctx object.Context, elems []object.Value, block object.Value) ([]object.Value, error) {
		out := make([]object.Value, 0, len(elems))
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	mapFn := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out, err := mapped(ctx, elems, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(out...), nil
	}
	withBlock("map", mapFn)
	withBlock("collect", mapFn)
	flatMap := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out := []object.Value{}
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if l, ok := v.AsList(); ok {
				out = append(out, l.Elems...)
			} else {
				out = append(out, v)
			}
		}
		return object.NewList(out...), nil
	}
	withBlock("flat_map", flatMap)
	withBlock("collect_concat", flatMap)
	withBlock("filter_map", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out := []object.Value{}
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if v.Truthy() {
				out = append(out, v)
			}
		}
		return object.NewList(out...), nil
	})
	selected := func(ctx object.Context, elems []object.Value, block object.Value, keep bool) ([]object.Value, error) {
		out := []object.Value{}
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return nil, err
			}
			if v.Truthy() == keep {
				out = append(out, el)
			}
		}
		return out, nil
	}
	selectFn := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out, err := selected(ctx, elems, block, true)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(out...), nil
	}
	withBlock("select", selectFn)
	withBlock("filter", selectFn)
	withBlock("find_all", selectFn)
	withBlock("reject", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		out, err := selected(ctx, elems, block, false)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(out...), nil
	})
	withBlock("partition", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		yes, no := []object.Value{}, []object.Value{}
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if v.Truthy() {
				yes = append(yes, el)
			} else {
				no = append(no, el)
			}
		}
		return object.NewList(object.NewList(yes...), object.NewList(no...)), nil
	})
	find := func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		for _, el := range elems {
			v, err := ctx.Yield(block, el)
			if err != nil {
				return object.Value{}, err
			}
			if v.Truthy() {
				return el, nil
			}
		}
		return object.Nil, nil
	}
	withBlock("find", find)
	withBlock("detect", find)
	each("find_index", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
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
		each(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
			n := -1
			if len(args) == 1 {
				k, err := wantInt(ctx, args[0], "")
				if err != nil {
					return object.Value{}, err
				}
				n = int(k)
			}
			sorted, err := sortValues(ctx, elems, block)
			if err != nil {
				return object.Value{}, err
			}
			if sign > 0 {
				reverse(sorted)
			}
			if n < 0 {
				if len(sorted) == 0 {
					return object.Nil, nil
				}
				return sorted[0], nil
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
		withBlock(name, func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
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
				n, err := wantInt(ctx, args[0], "")
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
	each("minmax", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		sorted, err := sortValues(ctx, elems, block)
		if err != nil {
			return object.Value{}, err
		}
		if len(sorted) == 0 {
			return object.NewList(object.Nil, object.Nil), nil
		}
		return object.NewList(sorted[0], sorted[len(sorted)-1]), nil
	})
	each("sort", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		sorted, err := sortValues(ctx, elems, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(sorted...), nil
	})
	withBlock("sort_by", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		sorted, err := sortBy(ctx, elems, block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(sorted...), nil
	})
	withBlock("group_by", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
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
			l, err := toList(ctx, a)
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
	withBlock("take_while", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
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
	withBlock("drop_while", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
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
			return object.Value{}, argError(ctx, len(args), "1")
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
				return object.Value{}, ctx.Raise(object.TypeError, "wrong element type %s (expected a [key, value] pair)", kindName(pair))
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
	// reverse_each returns its receiver, as each does; without a block it
	// returns the elements reversed.
	def(e, "reverse_each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems, err := toList(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			out := make([]object.Value, len(elems))
			for i, el := range elems {
				out[len(elems)-1-i] = el
			}
			return object.NewList(out...), nil
		}
		for i := len(elems) - 1; i >= 0; i-- {
			if _, err := ctx.Yield(block, elems[i]); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	each("each_entry", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		return yieldEach(ctx, elems, block)
	})
	withBlock("chunk_while", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		return chunks(ctx, elems, block, true)
	})
	withBlock("slice_when", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		return chunks(ctx, elems, block, false)
	})
	each("cycle", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "cycle"); err != nil {
			return object.Value{}, err
		}
		n, err := countArg(ctx, args, -1)
		if err != nil {
			return object.Value{}, err
		}
		if len(elems) == 0 {
			return object.Nil, nil
		}
		for round := 0; n < 0 || round < n; round++ {
			for _, el := range elems {
				if _, err := ctx.Yield(block, el); err != nil {
					return object.Value{}, err
				}
			}
		}
		return object.Nil, nil
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
	each("lazy", func(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
		return object.NewList(append([]object.Value{}, elems...)...), nil
	})
}

// inject is reduce with a symbol, an initial value, a block, or both.
func inject(ctx object.Context, elems []object.Value, args []object.Value, block object.Value) (object.Value, error) {
	var acc object.Value
	op := ""
	switch {
	case len(args) == 2:
		acc = args[0]
		s, err := wantString(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		op = s
	case len(args) == 1 && block.IsZero():
		s, err := wantString(ctx, args[0])
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
		return 0, argError(ctx, len(args), "1")
	}
	n, err := wantInt(ctx, args[0], "")
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
		return 0, argError(ctx, len(args), "1")
	}
	n, err := wantInt(ctx, args[0], "")
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

// sortValues sorts a copy of elems by <=>, or by a comparator block.
func sortValues(ctx object.Context, elems []object.Value, block object.Value) ([]object.Value, error) {
	out := append([]object.Value{}, elems...)
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
				sortErr = ctx.Raise(object.ArgumentError, "comparison of %s with %s failed", kindName(out[i]), kindName(out[j]))
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
	type keyed struct{ key, el object.Value }
	items := make([]keyed, len(elems))
	for i, el := range elems {
		k, err := ctx.Yield(block, el)
		if err != nil {
			return nil, err
		}
		items[i] = keyed{k, el}
	}
	var sortErr error
	sort.SliceStable(items, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		c, err := ctx.Compare(items[i].key, items[j].key)
		if err != nil {
			sortErr = err
			return false
		}
		return c < 0
	})
	if sortErr != nil {
		return nil, sortErr
	}
	out := make([]object.Value, len(items))
	for i, it := range items {
		out[i] = it.el
	}
	return out, nil
}

// chunks splits elems where the block, given two neighbours, returns
// false (chunk_while) or true (slice_when).
func chunks(ctx object.Context, elems []object.Value, block object.Value, together bool) (object.Value, error) {
	if len(elems) == 0 {
		return object.NewList(), nil
	}
	out := []object.Value{}
	cur := []object.Value{elems[0]}
	for i := 1; i < len(elems); i++ {
		r, err := ctx.Yield(block, elems[i-1], elems[i])
		if err != nil {
			return object.Value{}, err
		}
		if r.Truthy() == together {
			cur = append(cur, elems[i])
		} else {
			out = append(out, object.NewList(cur...))
			cur = []object.Value{elems[i]}
		}
	}
	out = append(out, object.NewList(cur...))
	return object.NewList(out...), nil
}

func grepElems(ctx object.Context, elems []object.Value, args []object.Value, block object.Value, keep bool) (object.Value, error) {
	if len(args) != 1 {
		return object.Value{}, argError(ctx, len(args), "1")
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
