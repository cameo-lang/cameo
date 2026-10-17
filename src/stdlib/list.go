package stdlib

import (
	"strings"

	"github.com/radiospiel/amber/src/object"
)

func installList() {
	c := object.ListClass
	// Array.new: empty, n nils, n copies of a value, n block values by
	// index, or a copy of another array.
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "0..2")
		}
		if len(args) == 0 {
			return object.NewList(), nil
		}
		if l, ok := args[0].AsList(); ok && len(args) == 1 {
			return object.NewList(append([]object.Value{}, l.Elems...)...), nil
		}
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative array size")
		}
		fill := object.Nil
		if len(args) == 2 {
			fill = args[1]
		}
		out := make([]object.Value, n)
		for i := range out {
			if !block.IsZero() {
				v, err := ctx.Yield(block, object.Int(i))
				if err != nil {
					return object.Value{}, err
				}
				out[i] = v
			} else {
				out[i] = fill
			}
		}
		return object.NewList(out...), nil
	})
	c.DefineSingleton("[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(append([]object.Value{}, args...)...), nil
	})
	list := func(self object.Value) *object.List { return self.List() }
	elems := func(self object.Value) []object.Value { return self.List().Elems }
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(list(self).Len()), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(list(self).Len() == 0), nil
	})
	def(c, "each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return self, nil
		}
		for _, e := range elems(self) {
			if _, err := ctx.Yield(block, e); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "each_index", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "each_index"); err != nil {
			return object.Value{}, err
		}
		for i := range elems(self) {
			if _, err := ctx.Yield(block, object.Int(i)); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return listIndex(ctx, list(self), args)
	})
	def(c, "slice", -1, c.Methods["[]"].Native)
	def(c, "at", 1, c.Methods["[]"].Native)
	def(c, "dig", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return dig(ctx, self, args)
	})
	def(c, "fetch", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		i, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		idx := int(i)
		n := list(self).Len()
		if idx < 0 {
			idx += n
		}
		if idx >= 0 && idx < n {
			return elems(self)[idx], nil
		}
		switch {
		case !block.IsZero():
			return ctx.Yield(block, args[0])
		case len(args) == 2:
			return args[1], nil
		}
		return object.Value{}, ctx.Raise(object.IndexError, "index %d outside of list bounds: %d...%d", i, -n, n)
	})
	def(c, "values_at", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, len(args))
		for i, a := range args {
			v, err := listIndex(ctx, list(self), []object.Value{a})
			if err != nil {
				return object.Value{}, err
			}
			out[i] = v
		}
		return object.NewList(out...), nil
	})
	def(c, "last", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := list(self).Len()
		if len(args) == 0 {
			if n == 0 {
				return object.Nil, nil
			}
			return elems(self)[n-1], nil
		}
		k, err := countArg(ctx, args, n)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(append([]object.Value{}, elems(self)[n-k:]...)...), nil
	})
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsList()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into List", kindName(args[0]))
		}
		return object.ListValue(concat(elems(self), o.Elems)), nil
	})
	def(c, "append", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ListValue(concat(elems(self), args)), nil
	})
	def(c, "prepend", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ListValue(concat(args, elems(self))), nil
	})
	def(c, "-", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.ListValue(without(elems(self), o)), nil
	})
	def(c, "without", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ListValue(without(elems(self), args)), nil
	})
	def(c, "without_at", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		drop := map[int]bool{}
		n := list(self).Len()
		for _, a := range args {
			i, err := wantInt(ctx, a, "")
			if err != nil {
				return object.Value{}, err
			}
			if i < 0 {
				i += int64(n)
			}
			drop[int(i)] = true
		}
		out := []object.Value{}
		for i, e := range elems(self) {
			if !drop[i] {
				out = append(out, e)
			}
		}
		return object.NewList(out...), nil
	})
	def(c, "*", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].Kind() {
		case object.KindInt:
			a := args[0].Int()
			if a < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative argument")
			}
			out := []object.Value{}
			for i := int64(0); i < int64(a); i++ {
				out = append(out, elems(self)...)
			}
			return object.NewList(out...), nil
		case object.KindString:
			a := args[0].Str()
			return join(ctx, elems(self), string(a))
		}
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(args[0]))
	})
	def(c, "&", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		set := object.NewSet(o...)
		out := []object.Value{}
		seen := map[string]bool{}
		for _, e := range elems(self) {
			if set.Has(e) && !seen[object.Key(e)] {
				seen[object.Key(e)] = true
				out = append(out, e)
			}
		}
		return object.NewList(out...), nil
	})
	def(c, "intersection", 1, c.Methods["&"].Native)
	def(c, "|", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(object.NewSet(concat(elems(self), o).Elems...).Elems()...), nil
	})
	def(c, "union", 1, c.Methods["|"].Native)
	def(c, "difference", 1, c.Methods["-"].Native)
	def(c, "intersect?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toList(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		set := object.NewSet(o...)
		for _, e := range elems(self) {
			if set.Has(e) {
				return object.True, nil
			}
		}
		return object.False, nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		return object.Bool(eq), err
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if _, ok := args[0].AsList(); !ok {
			return object.Nil, nil
		}
		r, err := ctx.Compare(self, args[0])
		if err != nil {
			return object.Nil, nil
		}
		return object.Int(r), nil
	})
	def(c, "reverse", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := append([]object.Value{}, elems(self)...)
		reverse(out)
		return object.NewList(out...), nil
	})
	def(c, "rotate", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := list(self).Len()
		if n == 0 {
			return self, nil
		}
		k := int64(1)
		if len(args) == 1 {
			var err error
			if k, err = wantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		k = ((k % int64(n)) + int64(n)) % int64(n)
		return object.ListValue(concat(elems(self)[k:], elems(self)[:k])), nil
	})
	def(c, "flatten", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		depth := int64(-1)
		if len(args) == 1 {
			var err error
			if depth, err = wantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
		}
		return object.NewList(flatten(elems(self), depth)...), nil
	})
	def(c, "compact", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := []object.Value{}
		for _, e := range elems(self) {
			if !e.IsNil() {
				out = append(out, e)
			}
		}
		return object.NewList(out...), nil
	})
	def(c, "join", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sep := ""
		if len(args) == 1 && !args[0].IsNil() {
			s, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			sep = s
		}
		return join(ctx, elems(self), sep)
	})
	def(c, "index", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for i, e := range elems(self) {
			hit, err := equalOrBlock(ctx, e, args, block)
			if err != nil {
				return object.Value{}, err
			}
			if hit {
				return object.Int(i), nil
			}
		}
		return object.Nil, nil
	})
	def(c, "rindex", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for i := list(self).Len() - 1; i >= 0; i-- {
			hit, err := equalOrBlock(ctx, elems(self)[i], args, block)
			if err != nil {
				return object.Value{}, err
			}
			if hit {
				return object.Int(i), nil
			}
		}
		return object.Nil, nil
	})
	def(c, "product", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		tuples := [][]object.Value{}
		for _, e := range elems(self) {
			tuples = append(tuples, []object.Value{e})
		}
		for _, a := range args {
			other, err := toList(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			next := [][]object.Value{}
			for _, t := range tuples {
				for _, o := range other {
					next = append(next, append(append([]object.Value{}, t...), o))
				}
			}
			tuples = next
		}
		out := make([]object.Value, len(tuples))
		for i, t := range tuples {
			out[i] = object.NewList(t...)
		}
		return object.NewList(out...), nil
	})
	def(c, "transpose", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		rows := elems(self)
		if len(rows) == 0 {
			return object.NewList(), nil
		}
		width := -1
		for _, r := range rows {
			l, ok := r.AsList()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into List", kindName(r))
			}
			if width >= 0 && l.Len() != width {
				return object.Value{}, ctx.Raise(object.IndexError, "element size differs (%d should be %d)", l.Len(), width)
			}
			width = l.Len()
		}
		out := make([]object.Value, width)
		for i := 0; i < width; i++ {
			col := make([]object.Value, len(rows))
			for j, r := range rows {
				col[j] = r.List().Elems[i]
			}
			out[i] = object.NewList(col...)
		}
		return object.NewList(out...), nil
	})
	// sample and shuffle draw from the interpreter's generator.
	def(c, "shuffle", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := append([]object.Value{}, elems(self)...)
		ctx.Rand().Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return object.NewList(out...), nil
	})
	def(c, "sample", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		es := elems(self)
		if len(args) == 0 {
			if len(es) == 0 {
				return object.Nil, nil
			}
			return es[ctx.Rand().IntN(len(es))], nil
		}
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		n, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative sample number")
		}
		// The first k steps of a Fisher-Yates shuffle over the indices,
		// with the swapped positions kept in a map, so a small sample of a
		// large array costs k, not the array's length.
		k := min(int(n), len(es))
		swapped := make(map[int]int, k)
		at := func(i int) int {
			if j, ok := swapped[i]; ok {
				return j
			}
			return i
		}
		out := make([]object.Value, k)
		for i := 0; i < k; i++ {
			j := i + ctx.Rand().IntN(len(es)-i)
			vi, vj := at(i), at(j)
			swapped[j] = vi
			out[i] = es[vj]
		}
		return object.NewList(out...), nil
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(c, "to_list", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "to_ary", 0, c.Methods["to_list"].Native)
	def(c, "pack", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "pack is not supported")
	})
	def(c, "each_slice", -1, object.Enumerable.Methods["each_slice"].Native)
	def(c, "sum", -1, object.Enumerable.Methods["sum"].Native)
	def(c, "assoc", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, e := range elems(self) {
			if pair, ok := e.AsList(); ok && pair.Len() > 0 {
				eq, err := ctx.Equal(pair.Elems[0], args[0])
				if err != nil {
					return object.Value{}, err
				}
				if eq {
					return object.ListValue(pair), nil
				}
			}
		}
		return object.Nil, nil
	})
	def(c, "combination", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		k, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		out := []object.Value{}
		var rec func(start int, cur []object.Value)
		rec = func(start int, cur []object.Value) {
			if int64(len(cur)) == k {
				out = append(out, object.NewList(append([]object.Value{}, cur...)...))
				return
			}
			for i := start; i < list(self).Len(); i++ {
				rec(i+1, append(cur, elems(self)[i]))
			}
		}
		if k >= 0 {
			rec(0, nil)
		}
		return yieldEach(ctx, out, block)
	})
	def(c, "sum_by", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Enumerable.Methods["sum"].Native(ctx, self, nil, nil, block)
	})
}

func concat(a, b []object.Value) *object.List {
	out := make([]object.Value, 0, len(a)+len(b))
	return object.NewList(append(append(out, a...), b...)...).List()
}

func without(elems, drop []object.Value) *object.List {
	set := object.NewSet(drop...)
	out := []object.Value{}
	for _, e := range elems {
		if !set.Has(e) {
			out = append(out, e)
		}
	}
	return object.NewList(out...).List()
}

func flatten(elems []object.Value, depth int64) []object.Value {
	out := []object.Value{}
	for _, e := range elems {
		if l, ok := e.AsList(); ok && depth != 0 {
			out = append(out, flatten(l.Elems, depth-1)...)
			continue
		}
		out = append(out, e)
	}
	return out
}

func join(ctx object.Context, elems []object.Value, sep string) (object.Value, error) {
	parts := make([]string, len(elems))
	for i, e := range elems {
		var s string
		var err error
		if l, ok := e.AsList(); ok {
			v, err := join(ctx, l.Elems, sep)
			if err != nil {
				return object.Value{}, err
			}
			s = string(v.Str())
		} else if s, err = ctx.ToS(e); err != nil {
			return object.Value{}, err
		}
		parts[i] = s
	}
	return object.String(strings.Join(parts, sep)), nil
}

func equalOrBlock(ctx object.Context, el object.Value, args []object.Value, block object.Value) (bool, error) {
	if len(args) == 1 {
		return ctx.Equal(el, args[0])
	}
	if block.IsZero() {
		return false, ctx.Raise(object.ArgumentError, "wrong number of arguments (given 0, expected 1)")
	}
	r, err := ctx.Yield(block, el)
	if err != nil {
		return false, err
	}
	return r.Truthy(), nil
}

// dig follows a path of indices and keys through nested lists and maps.
func dig(ctx object.Context, v object.Value, path []object.Value) (object.Value, error) {
	if len(path) == 0 {
		return object.Value{}, argError(ctx, 0, "1+")
	}
	for _, p := range path {
		switch v.Kind() {
		case object.KindList:
			cur := v.List()
			r, err := listIndex(ctx, cur, []object.Value{p})
			if err != nil {
				return object.Value{}, err
			}
			v = r
		case object.KindMap:
			cur := v.Map()
			r, ok := cur.Get(p)
			if !ok {
				return object.Nil, nil
			}
			v = r
		case object.KindInstance:
			cur := v.Instance()
			name, ok := p.AsSymbol()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "%s does not have #dig method", kindName(v))
			}
			f, ok := cur.Field(string(name))
			if !ok {
				return object.Nil, nil
			}
			if f.IsZero() {
				return object.Nil, nil
			}
			v = f
		default:
			if v.IsNil() {
				return object.Nil, nil
			}
			return object.Value{}, ctx.Raise(object.TypeError, "%s does not have #dig method", kindName(v))
		}
	}
	return v, nil
}

// listIndex is List#[] with an index, a start and length, or a range.
func listIndex(ctx object.Context, l *object.List, args []object.Value) (object.Value, error) {
	n := l.Len()
	switch len(args) {
	case 1:
		switch args[0].Kind() {
		case object.KindInt:
			a := args[0].Int()
			i := int(a)
			if i < 0 {
				i += n
			}
			if i < 0 || i >= n {
				return object.Nil, nil
			}
			return l.Elems[i], nil
		case object.KindRange:
			a := args[0].Range()
			start, length, ok := rangeBounds(ctx, a, n)
			if !ok {
				return object.Nil, nil
			}
			return object.NewList(append([]object.Value{}, l.Elems[start:start+length]...)...), nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", kindName(args[0]))
	case 2:
		start, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		length, err := wantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
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
	return object.Value{}, argError(ctx, len(args), "1..2")
}
