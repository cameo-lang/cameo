package stdlib

import "github.com/cameo-lang/cameo/src/object"

// installObject defines the methods every value has: BasicObject's, which
// are Ruby's BasicObject's, and Object's.
func installObject() {
	b := object.BasicObjectClass
	o := object.ObjectClass
	def(o, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(self)
		return object.String(s), err
	})
	def(o, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.Inspect(self)
		return object.String(s), err
	})
	def(b, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Equal(self, args[0])
		return object.Bool(eq), err
	})
	def(b, "!=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
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
	def(b, "!", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
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
		if ctx.RespondsTo(self, name) {
			return object.True, nil
		}
		// A class with method_missing says what else it answers.
		all := object.False
		if len(args) > 1 {
			all = object.Bool(args[1].Truthy())
		}
		r, err := ctx.Call(self, "respond_to_missing?", object.Symbol(name), all)
		return object.Bool(r.Truthy()), err
	})
	def(o, "respond_to_missing?", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.False, nil
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
	// clone(freeze: false) of a value Ruby always freezes, a number, a
	// Symbol, nil or a boolean, fails as in Ruby; of any other value it
	// returns the value, which stays frozen.
	def(o, "clone", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if kw == nil || kw.Len() == 0 {
			return self, nil
		}
		for _, k := range kw.Keys() {
			if k.Kind() != object.KindSymbol || k.Str() != "freeze" {
				s, err := ctx.Inspect(k)
				if err != nil {
					return object.Value{}, err
				}
				return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", s)
			}
		}
		v, _ := kw.Get(object.Symbol("freeze"))
		if v.Kind() != object.KindNil && v.Kind() != object.KindBool {
			return object.Value{}, ctx.Raise(object.ArgumentError, "unexpected value for freeze: %s", v.Class().Name)
		}
		if b, ok := v.AsBool(); ok && !b {
			switch self.Kind() {
			case object.KindInt, object.KindBig, object.KindFloat, object.KindSymbol, object.KindNil, object.KindBool:
				return object.Value{}, ctx.Raise(object.ArgumentError, "can't unfreeze %s", self.Class().Name)
			}
		}
		return self, nil
	})
	// then yields self; without a block it is an enumerator of self.
	then := func(name string) {
		def(o, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if block.IsZero() {
				return enumerator(false, self, name, nil, func(ctx object.Context, yield func(object.Value) error) error {
					return yield(self)
				}), nil
			}
			return ctx.Yield(block, self)
		})
	}
	then("then")
	then("yield_self")
	// to_enum and enum_for make an enumerator over what a method yields,
	// calling it again each time the enumerator runs.
	toEnum := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		meth := "each"
		if len(args) > 0 {
			name, ok := args[0].AsSymbol()
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "%s is not a symbol", kindName(args[0]))
			}
			meth, args = name, args[1:]
		}
		return enumerator(false, self, meth, args, func(ctx object.Context, yield func(object.Value) error) error {
			_, err := ctx.CallWith(self, meth, args, kw, ctx.Block(func(args []object.Value) (object.Value, error) {
				return object.Nil, yield(yielded(args))
			}))
			return err
		}), nil
	}
	def(o, "to_enum", -1, toEnum)
	def(o, "enum_for", -1, toEnum)
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
	// compare_by_identity returns the hash or set as it is: cameo values
	// have no identity to compare by, so equality is all there is.
	for _, c := range []*object.Class{object.MapClass, object.SetClass} {
		def(c, "compare_by_identity", 0, identity)
	}
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
