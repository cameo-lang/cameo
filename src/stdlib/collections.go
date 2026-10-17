package stdlib

import "github.com/radiospiel/amber/src/object"

// The collection classes get their full method sets with milestone 3. This
// file defines what the earlier examples need: size, indexing, + and put.

func installList() {
	c := object.ListClass
	list := func(self object.Value) *object.List { return self.(*object.List) }
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(list(self).Len()), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(list(self).Len() == 0), nil
	})
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(*object.List)
		if !ok {
			return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into List", kindName(args[0]))
		}
		elems := make([]object.Value, 0, list(self).Len()+o.Len())
		elems = append(append(elems, list(self).Elems...), o.Elems...)
		return object.NewList(elems...), nil
	})
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return listIndex(ctx, list(self), args)
	})
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each"); err != nil {
			return nil, err
		}
		for _, e := range list(self).Elems {
			if _, err := ctx.Yield(block, e); err != nil {
				return nil, err
			}
		}
		return self, nil
	})
}

// listIndex is List#[] with an index, a start and length, or a range.
func listIndex(ctx object.Context, l *object.List, args []object.Value) (object.Value, error) {
	n := l.Len()
	switch len(args) {
	case 1:
		switch a := args[0].(type) {
		case object.Int:
			i := int(a)
			if i < 0 {
				i += n
			}
			if i < 0 || i >= n {
				return object.Nil, nil
			}
			return l.Elems[i], nil
		case *object.Range:
			start, length, ok := rangeBounds(ctx, a, n)
			if !ok {
				return object.Nil, nil
			}
			return object.NewList(append([]object.Value{}, l.Elems[start:start+length]...)...), nil
		}
		return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(args[0]))
	case 2:
		start, err := wantInt(ctx, args[0], "")
		if err != nil {
			return nil, err
		}
		length, err := wantInt(ctx, args[1], "")
		if err != nil {
			return nil, err
		}
		if start < 0 {
			start += int64(n)
		}
		if start < 0 || start > int64(n) || length < 0 {
			return object.Nil, nil
		}
		end := start + length
		if end > int64(n) {
			end = int64(n)
		}
		return object.NewList(append([]object.Value{}, l.Elems[start:end]...)...), nil
	}
	return nil, argError(ctx, len(args), "1..2")
}

func installMap() {
	c := object.MapClass
	m := func(self object.Value) *object.Map { return self.(*object.Map) }
	def(c, "size", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(m(self).Len()), nil
	})
	def(c, "length", 0, c.Methods["size"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(m(self).Len() == 0), nil
	})
	def(c, "[]", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if v, ok := m(self).Get(args[0]); ok {
			return v, nil
		}
		return object.Nil, nil
	})
	def(c, "put", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return m(self).Put(args[0], args[1]), nil
	})
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each"); err != nil {
			return nil, err
		}
		for i, k := range m(self).Keys() {
			if _, err := ctx.Yield(block, k, m(self).Values()[i]); err != nil {
				return nil, err
			}
		}
		return self, nil
	})
}

func installSet() {}

func installRange() {
	c := object.RangeClass
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		elems, err := rangeElems(ctx, self.(*object.Range))
		if err != nil {
			return nil, err
		}
		return object.NewList(append([]object.Value{}, elems...)...), nil
	})
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each"); err != nil {
			return nil, err
		}
		elems, err := rangeElems(ctx, self.(*object.Range))
		if err != nil {
			return nil, err
		}
		for _, e := range elems {
			if _, err := ctx.Yield(block, e); err != nil {
				return nil, err
			}
		}
		return self, nil
	})
}

// rangeElems expands an integer range into its elements.
func rangeElems(ctx object.Context, r *object.Range) ([]object.Value, error) {
	lo, ok1 := r.Lo.(object.Int)
	hi, ok2 := r.Hi.(object.Int)
	if !ok1 || !ok2 {
		return nil, ctx.Raise(object.TypeError, "can't iterate over %s", kindName(r))
	}
	if r.Exclusive {
		hi--
	}
	out := []object.Value{}
	for i := lo; i <= hi; i++ {
		out = append(out, i)
	}
	return out, nil
}

func installEnumerable() {}
