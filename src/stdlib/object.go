package stdlib

import "github.com/radiospiel/amber/src/object"

// installObject defines the methods every value has.
func installObject() {
	o := object.ObjectClass
	def(o, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(self)
		return object.String(s), err
	})
	def(o, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.Inspect(self)
		return object.String(s), err
	})
	def(o, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		return object.Bool(eq), err
	})
	def(o, "!=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		return object.Bool(!eq), err
	})
	def(o, "eql?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Key(self) == object.Key(args[0])), nil
	})
	def(o, "===", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		return object.Bool(eq), err
	})
	def(o, "!", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(!self.Truthy()), nil
	})
	def(o, "class", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ClassValue(self.Class()), nil
	})
	isA := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "class or module required")
		}
		return object.Bool(self.Class().IsA(cls)), nil
	}
	def(o, "is_a?", 1, isA)
	def(o, "kind_of?", 1, isA)
	def(o, "instance_of?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, _ := args[0].AsClass()
		return object.Bool(self.Class() == c), nil
	})
	def(o, "nil?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.IsNil()), nil
	})
	def(o, "respond_to?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 {
			return object.Value{}, argError(ctx, 0, "1")
		}
		name, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(ctx.RespondsTo(self, name)), nil
	})
	def(o, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	def(o, "frozen?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.True, nil
	})
	identity := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	}
	def(o, "itself", 0, identity)
	def(o, "dup", 0, identity)
	def(o, "clone", 0, identity)
	def(o, "then", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "then"); err != nil {
			return object.Value{}, err
		}
		return ctx.Yield(block, self)
	})
	def(o, "instance_variables", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst, ok := self.AsInstance()
		if !ok {
			return object.NewList(), nil
		}
		out := make([]object.Value, len(inst.Cls.Fields))
		for i, f := range inst.Cls.Fields {
			out[i] = object.Symbol("@" + f.Name)
		}
		return object.NewList(out...), nil
	})
	def(o, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if eq {
			return object.Int(0), nil
		}
		return object.Nil, nil
	})
	def(o, "freeze", 0, identity)
	def(o, "to_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method 'to_proc' for %s", kindName(self))
	})
	comparable(object.Comparable)
}

// comparable defines the ordering methods over <=>.
func comparable(m *object.Class) {
	rel := func(name string, test func(int) bool) {
		def(m, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			c, err := ctx.Compare(self, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.Bool(test(c)), nil
		})
	}
	rel("<", func(c int) bool { return c < 0 })
	rel("<=", func(c int) bool { return c <= 0 })
	rel(">", func(c int) bool { return c > 0 })
	rel(">=", func(c int) bool { return c >= 0 })
	def(m, "between?", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		lo, err := ctx.Compare(self, args[0])
		if err != nil {
			return object.Value{}, err
		}
		hi, err := ctx.Compare(self, args[1])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(lo >= 0 && hi <= 0), nil
	})
	def(m, "clamp", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if c, err := ctx.Compare(self, args[0]); err != nil || c < 0 {
			return args[0], err
		}
		if c, err := ctx.Compare(self, args[1]); err != nil || c > 0 {
			return args[1], err
		}
		return self, nil
	})
}

func hashString(s string) int64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return int64(h >> 1)
}
