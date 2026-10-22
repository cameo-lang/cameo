package stdlib

import "github.com/cameo-lang/cameo/src/object"

func installSet() {
	c := object.SetClass
	set := func(self object.Value) *object.Set { return self.Set() }
	other := func(ctx object.Context, v object.Value) (*object.Set, error) {
		if s, ok := v.AsSet(); ok {
			return s, nil
		}
		elems, err := toList(ctx, v)
		if err != nil {
			return nil, err
		}
		return object.NewSet(elems...), nil
	}
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(set(self).Len()), nil
	})
	def(c, "length", 0, c.Methods["size"].Native)
	def(c, "count", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Enumerable.Methods["count"].Native(ctx, self, args, kw, block)
	})
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(set(self).Len() == 0), nil
	})
	has := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(set(self).Has(args[0])), nil
	}
	def(c, "include?", 1, has)
	def(c, "member?", 1, has)
	def(c, "===", 1, has)
	def(c, "contain?", 1, has)
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		for _, e := range set(self).Elems() {
			if _, err := ctx.Yield(block, e); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "add", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := set(self)
		for _, a := range args {
			out = out.With(a)
		}
		return object.SetValue(out), nil
	})
	def(c, "merge", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := set(self)
		for _, a := range args {
			elems, err := setElems(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			for _, e := range elems {
				out = out.With(e)
			}
		}
		return object.SetValue(out), nil
	})
	def(c, "flatten_merge", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := other(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.SetValue(flattenSet(o, set(self))), nil
	})
	def(c, "without", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := set(self)
		for _, a := range args {
			out = out.Without(a)
		}
		return object.SetValue(out), nil
	})
	binary := func(names []string, f func(a, b *object.Set) *object.Set) {
		for _, name := range names {
			def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
				o, err := other(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				return object.SetValue(f(set(self), o)), nil
			})
		}
	}
	binary([]string{"|", "union", "+"}, (*object.Set).Union)
	binary([]string{"&", "intersection"}, (*object.Set).Intersection)
	binary([]string{"-", "difference"}, (*object.Set).Difference)
	// ^ is in Ruby's order: the other's elements this set lacks, then this
	// set's the other lacks.
	binary([]string{"^"}, func(a, b *object.Set) *object.Set { return b.Difference(a).Union(a.Difference(b)) })
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsSet()
		if !ok {
			return object.Nil, nil
		}
		a := set(self)
		switch {
		case a.Len() == o.Len() && setSubset(a, o):
			return object.Int(0), nil
		case a.Len() < o.Len() && setSubset(a, o):
			return object.Int(-1), nil
		case a.Len() > o.Len() && setSubset(o, a):
			return object.Int(1), nil
		}
		return object.Nil, nil
	})
	def(c, "join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sep := ""
		if len(args) == 1 && !args[0].IsNil() {
			var err error
			if sep, err = wantString(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return join(ctx, set(self).Elems(), sep)
	})
	def(c, "flatten", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.SetValue(flattenSet(set(self), object.NewSet())), nil
	})
	// classify groups the members by the block's value, as a hash of sets.
	def(c, "classify", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "classify"); err != nil {
			return object.Value{}, err
		}
		groups := object.NewMap()
		for _, e := range set(self).Elems() {
			k, err := ctx.Yield(block, e)
			if err != nil {
				return object.Value{}, err
			}
			g, ok := groups.Get(k)
			if !ok {
				g = object.SetValue(object.NewSet())
			}
			groups = groups.Put(k, object.SetValue(g.Set().With(e)))
		}
		return object.MapValue(groups), nil
	})
	// divide is classify's sets with a one-argument block, and with a
	// two-argument one the sets of members the block relates both ways,
	// directly or through others. Without a block it is an enumerator
	// whose each divides by the block each takes.
	var divide fn
	divide = func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			e := enumerator(false, self, "divide", nil, func(ctx object.Context, yield func(object.Value) error) error {
				for _, el := range set(self).Elems() {
					if err := yield(el); err != nil {
						return err
					}
				}
				return nil
			})
			e.Enumerator().Call = func(ctx object.Context, block object.Value) (object.Value, error) {
				return divide(ctx, self, nil, nil, block)
			}
			return e, nil
		}
		arity, err := ctx.Call(block, "arity")
		if err != nil {
			return object.Value{}, err
		}
		elems := set(self).Elems()
		if n, _ := arity.AsInt(); n != 2 {
			groups, err := c.Methods["classify"].Native(ctx, self, nil, nil, block)
			if err != nil {
				return object.Value{}, err
			}
			return object.SetValue(object.NewSet(groups.Map().Values()...)), nil
		}
		// As in Ruby 3.4, the block sees every pair, a member with itself
		// too, and the sets are the graph's strongly connected components.
		edges := make([][]int, len(elems))
		for i := range elems {
			for j := range elems {
				r, err := ctx.Yield(block, elems[i], elems[j])
				if err != nil {
					return object.Value{}, err
				}
				if r.Truthy() {
					edges[i] = append(edges[i], j)
				}
			}
		}
		var out []object.Value
		for _, comp := range components(edges) {
			members := make([]object.Value, len(comp))
			for k, i := range comp {
				members[k] = elems[i]
			}
			out = append(out, object.SetValue(object.NewSet(members...)))
		}
		return object.SetValue(object.NewSet(out...)), nil
	}
	def(c, "divide", 0, divide)
	predicate := func(names []string, f func(a, b *object.Set) bool) {
		for _, name := range names {
			def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
				o, err := other(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				return object.Bool(f(set(self), o)), nil
			})
		}
	}
	subset := setSubset
	predicate([]string{"subset?", "<="}, subset)
	predicate([]string{"superset?", ">="}, func(a, b *object.Set) bool { return subset(b, a) })
	predicate([]string{"proper_subset?", "<"}, func(a, b *object.Set) bool { return subset(a, b) && a.Len() < b.Len() })
	predicate([]string{"proper_superset?", ">"}, func(a, b *object.Set) bool { return subset(b, a) && b.Len() < a.Len() })
	predicate([]string{"disjoint?"}, func(a, b *object.Set) bool { return a.Intersection(b).Len() == 0 })
	predicate([]string{"intersect?"}, func(a, b *object.Set) bool { return a.Intersection(b).Len() > 0 })
	predicate([]string{"=="}, func(a, b *object.Set) bool { return a.Len() == b.Len() && subset(a, b) })
	filter := func(name string, keep bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if err := needBlock(ctx, block, name); err != nil {
				return object.Value{}, err
			}
			out := []object.Value{}
			for _, e := range set(self).Elems() {
				r, err := ctx.Yield(block, e)
				if err != nil {
					return object.Value{}, err
				}
				if r.Truthy() == keep {
					out = append(out, e)
				}
			}
			return object.SetValue(object.NewSet(out...)), nil
		})
	}
	filter("select", true)
	filter("filter", true)
	filter("reject", false)
	def(c, "map", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Enumerable.Methods["map"].Native(ctx, self, args, kw, block)
	})
	def(c, "to_set", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		return object.Enumerable.Methods["to_set"].Native(ctx, self, args, kw, block)
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	object.SetClass.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if len(args) == 0 || args[0].IsNil() {
			return object.SetValue(object.NewSet()), nil
		}
		elems, err := setElems(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if !block.IsZero() {
			mapped := make([]object.Value, len(elems))
			for i, e := range elems {
				if mapped[i], err = ctx.Yield(block, e); err != nil {
					return object.Value{}, err
				}
			}
			elems = mapped
		}
		return object.SetValue(object.NewSet(elems...)), nil
	})
	object.SetClass.DefineSingleton("[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.SetValue(object.NewSet(args...)), nil
	})
}

func setSubset(a, b *object.Set) bool {
	for _, e := range a.Elems() {
		if !b.Has(e) {
			return false
		}
	}
	return true
}

// setElems is the elements Set.new and merge take from v: those of an
// Enumerable, or an ArgumentError for anything without each.
func setElems(ctx object.Context, v object.Value) ([]object.Value, error) {
	switch v.Kind() {
	case object.KindList, object.KindSet, object.KindMap, object.KindRange, object.KindEnumerator:
		return toList(ctx, v)
	}
	if v.Kind() == object.KindInstance && ctx.RespondsTo(v, "each") {
		return toList(ctx, v)
	}
	return nil, ctx.Raise(object.ArgumentError, "value must be enumerable")
}

// flattenSet adds the members of s to out, those of a member set in its
// place.
func flattenSet(s, out *object.Set) *object.Set {
	for _, e := range s.Elems() {
		if inner, ok := e.AsSet(); ok {
			out = flattenSet(inner, out)
			continue
		}
		out = out.With(e)
	}
	return out
}

// components returns the strongly connected components of the graph of
// edges, each in the order its nodes were reached, in the order Tarjan's
// algorithm, as Ruby's TSort runs it, completes them.
func components(edges [][]int) [][]int {
	index := make([]int, len(edges))
	low := make([]int, len(edges))
	onStack := make([]bool, len(edges))
	for i := range index {
		index[i] = -1
	}
	var stack []int
	var out [][]int
	next := 0
	var visit func(v int)
	visit = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range edges[v] {
			switch {
			case index[w] < 0:
				visit(w)
				low[v] = min(low[v], low[w])
			case onStack[w]:
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		at := len(stack) - 1
		for stack[at] != v {
			at--
		}
		comp := append([]int{}, stack[at:]...)
		for _, w := range comp {
			onStack[w] = false
		}
		stack = stack[:at]
		out = append(out, comp)
	}
	for v := range edges {
		if index[v] < 0 {
			visit(v)
		}
	}
	return out
}
