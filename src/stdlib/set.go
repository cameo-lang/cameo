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
		var flat func(s *object.Set, out *object.Set) *object.Set
		flat = func(s *object.Set, out *object.Set) *object.Set {
			for _, e := range s.Elems() {
				if inner, ok := e.AsSet(); ok {
					out = flat(inner, out)
					continue
				}
				out = out.With(e)
			}
			return out
		}
		return object.SetValue(flat(set(self), object.NewSet())), nil
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
	// two-argument one the sets of members the block relates, directly or
	// through others.
	def(c, "divide", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "divide"); err != nil {
			return object.Value{}, err
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
		parent := make([]int, len(elems))
		for i := range parent {
			parent[i] = i
		}
		var find func(i int) int
		find = func(i int) int {
			for parent[i] != i {
				parent[i] = parent[parent[i]]
				i = parent[i]
			}
			return i
		}
		for i := range elems {
			for j := range elems {
				if i == j || find(i) == find(j) {
					continue
				}
				r, err := ctx.Yield(block, elems[i], elems[j])
				if err != nil {
					return object.Value{}, err
				}
				if r.Truthy() {
					parent[find(j)] = find(i)
				}
			}
		}
		var order []int
		members := map[int][]object.Value{}
		for i, e := range elems {
			root := find(i)
			if _, seen := members[root]; !seen {
				order = append(order, root)
			}
			members[root] = append(members[root], e)
		}
		out := make([]object.Value, len(order))
		for k, root := range order {
			out[k] = object.SetValue(object.NewSet(members[root]...))
		}
		return object.SetValue(object.NewSet(out...)), nil
	})
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
		return self, nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	object.SetClass.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.SetValue(object.NewSet()), nil
		}
		elems, err := toList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
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
