package stdlib

import (
	"strings"

	"github.com/radiospiel/amber/src/object"
)

func installSymbol() {
	c := object.SymbolClass
	sym := func(self object.Value) string { return string(self.(object.Symbol)) }
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(sym(self)), nil
	})
	def(c, "name", 0, c.Methods["to_s"].Native)
	def(c, "id2name", 0, c.Methods["to_s"].Native)
	def(c, "to_sym", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(object.SymbolInspect(sym(self))), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(object.Symbol)
		return object.BoolOf(ok && o == self.(object.Symbol)), nil
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(object.Symbol)
		if !ok {
			return object.Nil, nil
		}
		return object.IntValue(strings.Compare(sym(self), string(o))), nil
	})
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(len([]rune(sym(self)))), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(sym(self) == ""), nil
	})
	mapSym := func(name string, f func(string) string) {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Symbol(f(sym(self))), nil
		})
	}
	mapSym("upcase", strings.ToUpper)
	mapSym("downcase", strings.ToLower)
	mapSym("capitalize", func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	})
	mapSym("succ", succ)
	strArg := func(name string, f func(s, a string) bool) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, err := wantString(ctx, args[0])
			if err != nil {
				return nil, err
			}
			return object.BoolOf(f(sym(self), a)), nil
		})
	}
	strArg("start_with?", strings.HasPrefix)
	strArg("end_with?", strings.HasSuffix)
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return stringIndex(ctx, sym(self), args)
	})
	def(c, "to_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return nil, ctx.Raise(object.NotImplementedErr, "Symbol#to_proc is written &:%s", sym(self))
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(hashString(object.Key(self))), nil
	})
}

func installNilBool() {
	n := object.NilClass
	def(n, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(""), nil
	})
	def(n, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(), nil
	})
	def(n, "to_h", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewMap(), nil
	})
	def(n, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.IntValue(0), nil
	})
	def(n, "to_f", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(0), nil
	})
	def(n, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("nil"), nil
	})
	def(n, "&", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.False, nil
	})
	def(n, "|", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Truthy(args[0])), nil
	})
	b := object.BoolClass
	def(b, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if object.Truthy(self) {
			return object.String("true"), nil
		}
		return object.String("false"), nil
	})
	def(b, "inspect", 0, b.Methods["to_s"].Native)
	def(b, "&", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Truthy(self) && object.Truthy(args[0])), nil
	})
	def(b, "|", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Truthy(self) || object.Truthy(args[0])), nil
	})
	def(b, "^", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(object.Truthy(self) != object.Truthy(args[0])), nil
	})
	object.ObjectClass.Consts["TrueClass"] = b
	object.ObjectClass.Consts["FalseClass"] = b
}

func installException() {
	c := object.ExceptionClass
	exc := func(self object.Value) *object.Exception { return self.(*object.Exception) }
	def(c, "message", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(exc(self).Message), nil
	})
	def(c, "to_s", 0, c.Methods["message"].Native)
	def(c, "full_message", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(exc(self).Message + " (" + exc(self).Cls.QualifiedName() + ")"), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("#<" + exc(self).Cls.QualifiedName() + ": " + exc(self).Message + ">"), nil
	})
	def(c, "backtrace", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	def(c, "cause", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].(*object.Exception)
		return object.BoolOf(ok && o.Cls == exc(self).Cls && o.Message == exc(self).Message), nil
	})
	def(c, "exception", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return self, nil
		}
		msg, err := ctx.ToS(args[0])
		if err != nil {
			return nil, err
		}
		return &object.Exception{Cls: exc(self).Cls, Message: msg}, nil
	})
	newExc := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.(*object.Class)
		msg := cls.QualifiedName()
		if len(args) > 1 {
			return nil, argError(ctx, len(args), "0..1")
		}
		if len(args) == 1 {
			s, err := ctx.ToS(args[0])
			if err != nil {
				return nil, err
			}
			msg = s
		}
		return &object.Exception{Cls: cls, Message: msg}, nil
	}
	c.DefineSingleton("new", -1, newExc)
	c.DefineSingleton("exception", -1, newExc)
}
