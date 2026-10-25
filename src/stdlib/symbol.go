package stdlib

import (
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

func installSymbol() {
	c := object.SymbolClass
	sym := func(self object.Value) string { return string(self.Str()) }
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return symString(self), nil
	})
	def(c, "encoding", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return encodingValue(symKind(sym(self))), nil
	})
	def(c, "name", 0, c.Methods["to_s"].Native)
	def(c, "id2name", 0, c.Methods["to_s"].Native)
	def(c, "to_sym", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.StringOf(symKind(sym(self)), object.SymbolInspect(sym(self))), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsSymbol()
		return object.Bool(ok && o == self.Str()), nil
	})
	def(c, "<=>", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsSymbol()
		if !ok {
			return object.Nil, nil
		}
		return object.Int(strings.Compare(sym(self), string(o))), nil
	})
	def(c, "length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(len([]rune(sym(self)))), nil
	})
	def(c, "size", 0, c.Methods["length"].Native)
	def(c, "empty?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(sym(self) == ""), nil
	})
	// The case methods are String's, in the symbol's encoding.
	for _, name := range []string{"upcase", "downcase", "capitalize", "swapcase"} {
		def(c, name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			v, err := object.StringClass.Methods[name].Native(ctx, symString(self), args, kw, block)
			if err != nil {
				return object.Value{}, err
			}
			return object.Symbol(v.Str()), nil
		})
	}
	def(c, "succ", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Symbol(succ(sym(self))), nil
	})
	// casecmp and casecmp? compare as String's do, and with anything but
	// a Symbol return nil.
	def(c, "casecmp", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if _, ok := args[0].AsSymbol(); !ok {
			return object.Nil, nil
		}
		return casecmp(symString(self), symString(args[0])), nil
	})
	def(c, "casecmp?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if _, ok := args[0].AsSymbol(); !ok {
			return object.Nil, nil
		}
		return casecmpP(symString(self), symString(args[0])), nil
	})
	strArg := func(name string, f func(s, a string) bool) {
		def(c, name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, err := wantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return object.Bool(f(sym(self), a)), nil
		})
	}
	strArg("start_with?", strings.HasPrefix)
	strArg("end_with?", strings.HasSuffix)
	def(c, "[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return stringIndex(ctx, sym(self), args)
	})
	def(c, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
}

// symKind is the encoding of a symbol named s. A symbol keeps no
// encoding of its own: one of ASCII alone is US-ASCII, as in Ruby, other
// valid UTF-8 is UTF-8, and any other bytes are binary.
func symKind(s string) object.Kind {
	switch {
	case object.ASCIIOnly(s):
		return object.KindASCII
	case utf8.ValidString(s):
		return object.KindString
	}
	return object.KindBytes
}

// symString is a symbol's name as a String in the symbol's encoding.
func symString(v object.Value) object.Value { return object.StringOf(symKind(v.Str()), v.Str()) }

func installNilBool() {
	n := object.NilClass
	def(n, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(""), nil
	})
	def(n, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(), nil
	})
	def(n, "to_h", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.MapValue(object.NewMap()), nil
	})
	def(n, "to_i", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(0), nil
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
		return object.Bool(args[0].Truthy()), nil
	})
	b := object.BoolClass
	def(b, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if self.Truthy() {
			return object.String("true"), nil
		}
		return object.String("false"), nil
	})
	def(b, "inspect", 0, b.Methods["to_s"].Native)
	def(b, "&", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.Truthy() && args[0].Truthy()), nil
	})
	def(b, "|", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.Truthy() || args[0].Truthy()), nil
	})
	def(b, "^", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(self.Truthy() != args[0].Truthy()), nil
	})
	object.ObjectClass.Consts["TrueClass"] = object.ClassValue(b)
	object.ObjectClass.Consts["FalseClass"] = object.ClassValue(b)
}

func installException() {
	c := object.ExceptionClass
	exc := func(self object.Value) *object.Exception { return self.Exception() }
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
		o, ok := args[0].AsException()
		return object.Bool(ok && o.Cls == exc(self).Cls && o.Message == exc(self).Message), nil
	})
	def(c, "exception", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return self, nil
		}
		msg, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.ExceptionValue(&object.Exception{Cls: exc(self).Cls, Message: msg}), nil
	})
	newExc := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		msg := cls.QualifiedName()
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if len(args) == 1 {
			s, err := ctx.ToS(args[0])
			if err != nil {
				return object.Value{}, err
			}
			msg = s
		}
		return object.ExceptionValue(&object.Exception{Cls: cls, Message: msg}), nil
	}
	c.DefineSingleton("new", -1, newExc)
	c.DefineSingleton("exception", -1, newExc)
}
