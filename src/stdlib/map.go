package stdlib

import "github.com/cameo-lang/cameo/src/object"

func installMap() {
	c := object.MapClass
	// Hash.new takes a default value, or a block that computes the default
	// from the hash and the key. The block cannot store what it computes,
	// since the hash is immutable; the syntax check rejects a block that
	// tries.
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if kw != nil && kw.Len() > 0 {
			s, err := ctx.Inspect(kw.Keys()[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
		}
		switch {
		case len(args) > 1:
			return object.Value{}, argError(ctx, len(args), "0..1")
		case len(args) == 1 && !block.IsZero():
			return object.Value{}, argError(ctx, 1, "0")
		case !block.IsZero():
			return object.MapValue(object.NewMap().WithDefault(object.Value{}, block)), nil
		case len(args) == 1:
			return object.MapValue(object.NewMap().WithDefault(args[0], object.Value{})), nil
		}
		return object.MapValue(object.NewMap()), nil
	})
	m := func(self object.Value) *object.Map { return self.Map() }
	pairs := func(self object.Value) []object.Value {
		out := make([]object.Value, m(self).Len())
		for i, k := range m(self).Keys() {
			out[i] = object.NewList(k, m(self).Values()[i])
		}
		return out
	}
	def(c, "deconstruct_keys", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
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
		return mapGet(ctx, self, args[0])
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
		return object.Value{}, &object.Exception{Cls: object.KeyError, Message: "key not found: " + s,
			Attrs: map[string]object.Value{"key": args[0], "receiver": self}}
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
			v, err := mapGet(ctx, self, a)
			if err != nil {
				return object.Value{}, err
			}
			out[i] = v
		}
		return object.NewList(out...), nil
	})
	def(c, "put", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.MapValue(m(self).Put(args[0], args[1])), nil
	})
	def(c, "merge", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// `h.merge(a: 1)` passes its hash as keywords.
		if kw != nil && kw.Len() > 0 {
			args = append(args, object.MapValue(kw))
		}
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
		return object.MapValue(keepDefault(m(self), out)), nil
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
			if block.IsZero() {
				return blockless(self, name, nil, true), nil
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
		if block.IsZero() {
			return blockless(self, "transform_values", nil, true), nil
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
		if mapping == nil && len(args) == 0 && block.IsZero() {
			return blockless(self, "transform_keys", nil, true), nil
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
	// flatten is to_a.flatten(depth), one level by default.
	def(c, "flatten", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		depth := int64(1)
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if len(args) == 1 {
			var err error
			if depth, err = wantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		if depth == 0 {
			return object.NewList(pairs(self)...), nil
		}
		// One level goes into flattening the pairs themselves, without
		// building them.
		out := make([]object.Value, 0, 2*m(self).Len())
		for i, k := range m(self).Keys() {
			for _, e := range []object.Value{k, m(self).Values()[i]} {
				if l, ok := e.AsList(); ok && depth != 1 {
					out = append(out, flatten(l.Elems, depth-2)...)
					continue
				}
				out = append(out, e)
			}
		}
		return object.NewList(out...), nil
	})
	def(c, "compact", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b := object.NewMapBuilder()
		for i, k := range m(self).Keys() {
			if v := m(self).Values()[i]; !v.IsNil() {
				b.Set(k, v)
			}
		}
		return object.MapValue(keepDefault(m(self), b.Map())), nil
	})
	eachPair := func(name string) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if block.IsZero() {
				return blockless(self, name, nil, true), nil
			}
			for i, k := range m(self).Keys() {
				if _, err := ctx.Yield(block, k, m(self).Values()[i]); err != nil {
					return object.Value{}, err
				}
			}
			return self, nil
		})
	}
	eachPair("each")
	eachPair("each_pair")
	def(c, "each_key", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return blockless(self, "each_key", nil, true), nil
		}
		for _, k := range m(self).Keys() {
			if _, err := ctx.Yield(block, k); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "each_value", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return blockless(self, "each_value", nil, true), nil
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
		sorted, err := sortValues(ctx, pairs(self), 0, block)
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
	// default is the default value, or with a key the default for that key,
	// which a default proc computes.
	def(c, "default", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch len(args) {
		case 0:
			value, _ := m(self).Default()
			return object.OrNil(value), nil
		case 1:
			return mapDefault(ctx, self, args[0])
		}
		return object.Value{}, argError(ctx, len(args), "0..1")
	})
	def(c, "default_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		_, proc := m(self).Default()
		return object.OrNil(proc), nil
	})
}

// mapGet is h[key]: the entry, or the default for a key h does not hold.
func mapGet(ctx object.Context, h object.Value, key object.Value) (object.Value, error) {
	if v, ok := h.Map().Get(key); ok {
		return v, nil
	}
	return mapDefault(ctx, h, key)
}

// mapDefault is the default for key: the default proc's result for the
// hash and key, the default value, or nil.
func mapDefault(ctx object.Context, h object.Value, key object.Value) (object.Value, error) {
	value, proc := h.Map().Default()
	if !proc.IsZero() {
		return ctx.Yield(proc, h, key)
	}
	return object.OrNil(value), nil
}

// keepDefault gives out the default of from, for a method that keeps it,
// as merge and compact do in Ruby.
func keepDefault(from, out *object.Map) *object.Map {
	value, proc := from.Default()
	if v, p := out.Default(); object.Same(v, value) && object.Same(p, proc) {
		return out
	}
	return out.WithDefault(value, proc)
}
