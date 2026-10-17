package stdlib

import "github.com/radiospiel/amber/src/object"

func installBytes() {
	c := object.BytesClass
	bytes := func(self object.Value) []byte { return []byte(self.(object.Bytes)) }
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(len(bytes(self))), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "bytesize", 0, c.Methods["length"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(len(bytes(self)) == 0), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(object.Bytes)
		return object.BoolOf(ok && string(o) == string(bytes(self))), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(bytes(self)), nil
	})
	def(c, "to_str", 0, c.Methods["to_s"].Native)
	def(c, "b", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(object.QuoteString(string(bytes(self))) + ".b"), nil
	})
	def(c, "bytes", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, len(bytes(self)))
		for i, b := range bytes(self) {
			out[i] = object.IntValue(b)
		}
		return object.NewList(out...), nil
	})
	def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(object.Bytes)
		if !ok {
			return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into Bytes", kindName(args[0]))
		}
		return object.Bytes(append(append([]byte{}, bytes(self)...), o...)), nil
	})
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 1 {
			if i, ok := args[0].(object.Int); ok {
				idx := int(i)
				if idx < 0 {
					idx += len(bytes(self))
				}
				if idx < 0 || idx >= len(bytes(self)) {
					return object.Nil, nil
				}
				return object.IntValue(bytes(self)[idx]), nil
			}
		}
		return nil, ctx.Raise(object.TypeError, "bytes are indexed by an integer")
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(hashString(object.Key(self))), nil
	})
}
