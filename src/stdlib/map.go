package stdlib

import "github.com/radiospiel/amber/src/object"

func installMap() {
	c := object.MapClass
	m := func(self object.Value) *object.Map { return self.Map() }
	pairs := func(self object.Value) []object.Value {
		out := make([]object.Value, m(self).Len())
		for i, k := range m(self).Keys() {
			out[i] = object.NewList(k, m(self).Values()[i])
		}
		return out
	}
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(m(self).Len()), nil
	})
	def(c, "length", 0, c.Methods["size"].Native)
	def(c, "count", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Enumerable.Methods["count"].Native(ctx, self, args, kw, block)
	})
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(m(self).Len() == 0), nil
	})
	def(c, "[]", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if v, ok := m(self).Get(args[0]); ok {
			return v, nil
		}
		return object.Nil, nil
	})
	def(c, "fetch", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		if v, ok := m(self).Get(args[0]); ok {
			return v, nil
		}
		switch {
		case !block.IsZero():
			return ctx.Yield(block, args[0])
		case len(args) == 2:
			return args[1], nil
		}
		s, _ := ctx.Inspect(args[0])
		return object.Value{}, ctx.Raise(object.KeyError, "key not found: %s", s)
	})
	def(c, "dig", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return dig(ctx, self, args)
	})
	hasKey := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(m(self).Has(args[0])), nil
	}
	def(c, "key?", 1, hasKey)
	def(c, "has_key?", 1, hasKey)
	def(c, "include?", 1, hasKey)
	def(c, "member?", 1, hasKey)
	hasValue := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, v := range m(self).Values() {
			eq, err := ctx.Equal(v, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if eq {
				return object.True, nil
			}
		}
		return object.False, nil
	}
	def(c, "value?", 1, hasValue)
	def(c, "has_value?", 1, hasValue)
	def(c, "key", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for i, v := range m(self).Values() {
			eq, err := ctx.Equal(v, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if eq {
				return m(self).Keys()[i], nil
			}
		}
		return object.Nil, nil
	})
	def(c, "keys", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(append([]object.Value{}, m(self).Keys()...)...), nil
	})
	def(c, "values", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(append([]object.Value{}, m(self).Values()...)...), nil
	})
	def(c, "values_at", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, len(args))
		for i, a := range args {
			v, ok := m(self).Get(a)
			if !ok {
				v = object.Nil
			}
			out[i] = v
		}
		return object.NewList(out...), nil
	})
	def(c, "put", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.MapValue(m(self).Put(args[0], args[1])), nil
	})
	def(c, "merge", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := m(self)
		for _, a := range args {
			other, ok := a.AsMap()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Map", kindName(a))
			}
			if block.IsZero() {
				out = out.Merge(other)
				continue
			}
			b := object.NewMapBuilder()
			for i, k := range out.Keys() {
				b.Set(k, out.Values()[i])
			}
			for i, k := range other.Keys() {
				v := other.Values()[i]
				if old, ok := out.Get(k); ok {
					r, err := ctx.Yield(block, k, old, v)
					if err != nil {
						return object.Value{}, err
					}
					v = r
				}
				b.Set(k, v)
			}
			out = b.Map()
		}
		return object.MapValue(out), nil
	})
	def(c, "without", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.MapValue(m(self).Without(args...)), nil
	})
	def(c, "except", -1, c.Methods["without"].Native)
	def(c, "slice", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for _, k := range args {
			if v, ok := m(self).Get(k); ok {
				b.Set(k, v)
			}
		}
		return object.MapValue(b.Map()), nil
	})
	filter := func(name string, keep bool) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if err := needBlock(ctx, block, name); err != nil {
				return object.Value{}, err
			}
			b := object.NewMapBuilder()
			for i, k := range m(self).Keys() {
				v := m(self).Values()[i]
				r, err := ctx.Yield(block, k, v)
				if err != nil {
					return object.Value{}, err
				}
				if r.Truthy() == keep {
					b.Set(k, v)
				}
			}
			return object.MapValue(b.Map()), nil
		})
	}
	filter("select", true)
	filter("filter", true)
	filter("reject", false)
	def(c, "transform_values", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "transform_values"); err != nil {
			return object.Value{}, err
		}
		b := object.NewMapBuilder()
		for i, k := range m(self).Keys() {
			r, err := ctx.Yield(block, m(self).Values()[i])
			if err != nil {
				return object.Value{}, err
			}
			b.Set(k, r)
		}
		return object.MapValue(b.Map()), nil
	})
	def(c, "transform_keys", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var mapping *object.Map
		if len(args) == 1 {
			mapping, _ = args[0].AsMap()
		}
		if mapping == nil {
			if err := needBlock(ctx, block, "transform_keys"); err != nil {
				return object.Value{}, err
			}
		}
		b := object.NewMapBuilder()
		for i, k := range m(self).Keys() {
			nk := k
			if mapping != nil {
				if v, ok := mapping.Get(k); ok {
					nk = v
				}
			} else {
				r, err := ctx.Yield(block, k)
				if err != nil {
					return object.Value{}, err
				}
				nk = r
			}
			b.Set(nk, m(self).Values()[i])
		}
		return object.MapValue(b.Map()), nil
	})
	def(c, "invert", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for i, k := range m(self).Keys() {
			b.Set(m(self).Values()[i], k)
		}
		return object.MapValue(b.Map()), nil
	})
	def(c, "compact", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for i, k := range m(self).Keys() {
			if v := m(self).Values()[i]; !v.IsNil() {
				b.Set(k, v)
			}
		}
		return object.MapValue(b.Map()), nil
	})
	eachPair := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		for i, k := range m(self).Keys() {
			if _, err := ctx.Yield(block, k, m(self).Values()[i]); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	}
	def(c, "each", 0, eachPair)
	def(c, "each_pair", 0, eachPair)
	def(c, "each_key", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each_key"); err != nil {
			return object.Value{}, err
		}
		for _, k := range m(self).Keys() {
			if _, err := ctx.Yield(block, k); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "each_value", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each_value"); err != nil {
			return object.Value{}, err
		}
		for _, v := range m(self).Values() {
			if _, err := ctx.Yield(block, v); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "to_h", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		return object.Enumerable.Methods["to_h"].Native(ctx, self, args, kw, block)
	})
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(pairs(self)...), nil
	})
	def(c, "sort", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sorted, err := sortValues(ctx, pairs(self), block)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(sorted...), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, ok := args[0].AsMap()
		if !ok || other.Len() != m(self).Len() {
			return object.False, nil
		}
		for i, k := range m(self).Keys() {
			ov, found := other.Get(k)
			if !found {
				return object.False, nil
			}
			eq, err := ctx.Equal(m(self).Values()[i], ov)
			if err != nil {
				return object.Value{}, err
			}
			if !eq {
				return object.False, nil
			}
		}
		return object.True, nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "any?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 && block.IsZero() {
			return object.Bool(m(self).Len() > 0), nil
		}
		return object.Enumerable.Methods["any?"].Native(ctx, self, args, kw, block)
	})
	def(c, "sum", -1, object.Enumerable.Methods["sum"].Native)
	def(c, "min_by", -1, object.Enumerable.Methods["min_by"].Native)
	def(c, "max_by", -1, object.Enumerable.Methods["max_by"].Native)
	def(c, "find", 0, object.Enumerable.Methods["find"].Native)
	def(c, "map", 0, object.Enumerable.Methods["map"].Native)
	def(c, "flat_map", 0, object.Enumerable.Methods["flat_map"].Native)
	def(c, "filter_map", 0, object.Enumerable.Methods["filter_map"].Native)
	def(c, "group_by", 0, object.Enumerable.Methods["group_by"].Native)
	def(c, "partition", 0, object.Enumerable.Methods["partition"].Native)
	def(c, "sort_by", 0, object.Enumerable.Methods["sort_by"].Native)
	def(c, "each_with_object", -1, object.Enumerable.Methods["each_with_object"].Native)
	def(c, "each_with_index", 0, object.Enumerable.Methods["each_with_index"].Native)
	def(c, "reduce", -1, object.Enumerable.Methods["reduce"].Native)
	def(c, "inject", -1, object.Enumerable.Methods["inject"].Native)
	def(c, "all?", -1, object.Enumerable.Methods["all?"].Native)
	def(c, "none?", -1, object.Enumerable.Methods["none?"].Native)
	def(c, "find_all", 0, object.Enumerable.Methods["find_all"].Native)
	def(c, "to_set", 0, object.Enumerable.Methods["to_set"].Native)
	def(c, "first", -1, object.Enumerable.Methods["first"].Native)
	def(c, "default", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
}
