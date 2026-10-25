package stdlib

import (
	"github.com/cameo-lang/cameo/src/object"
)

// Exception's methods. new is the evaluator's: it makes the exception
// with the class's name as its message and runs initialize on it, a
// script's own or the ones below, which `super` from a script's reaches.
// An initialize sets the exception it is given, which new alone hands it.

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
		return object.OrNil(exc(self).Cause), nil
	})
	def(c, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, ok := args[0].AsException()
		e := exc(self)
		if !ok || o.Cls != e.Cls || o.Message != e.Message || len(o.Fields) != len(e.Fields) {
			return object.False, nil
		}
		for i := range e.Fields {
			eq, err := ctx.Equal(object.OrNil(e.Fields[i]), object.OrNil(o.Fields[i]))
			if err != nil || !eq {
				return object.False, err
			}
		}
		return object.True, nil
	})
	def(c, "exception", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return self, nil
		}
		msg, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		out := exc(self).Copy()
		out.Message, out.Line = msg, 0
		return object.ExceptionValue(out), nil
	})
	// Exception#initialize(msg = nil)
	def(c, "initialize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		e, err := building(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if err := keywords(ctx, e, kw); err != nil {
			return object.Value{}, err
		}
		if e.Message, err = message(ctx, e.Cls, args, 0); err != nil {
			return object.Value{}, err
		}
		return object.Nil, nil
	})
	installExceptionAttrs()
}

// building returns the exception an initialize sets, and an error when
// self is an exception new is not making: an exception never changes once
// made.
func building(ctx object.Context, self object.Value) (*object.Exception, error) {
	e, ok := self.AsException()
	if !ok || !e.Building {
		return nil, ctx.Raise(object.CameoError, "initialize runs only in new")
	}
	return e, nil
}

// message is an initialize's message argument: its to_s, or the class's
// name when it is absent or nil.
func message(ctx object.Context, cls *object.Class, args []object.Value, i int) (string, error) {
	if i >= len(args) || args[i].IsNil() {
		return cls.QualifiedName(), nil
	}
	return ctx.ToS(args[i])
}

// keywords sets the attributes named among an initialize's keywords. The
// rest are an error for an exception whose class declares fields, since
// new took the fields' keywords already, and are left out otherwise.
func keywords(ctx object.Context, e *object.Exception, kw *object.Map, names ...string) error {
	if kw == nil {
		return nil
	}
	for i, k := range kw.Keys() {
		name, ok := k.AsFieldName()
		if !ok {
			if len(e.Cls.Fields) > 0 {
				return ctx.Raise(object.ArgumentError, "%s.new: fields are named by symbols", e.Cls.QualifiedName())
			}
			continue
		}
		known := false
		for _, n := range names {
			if n == name {
				known = true
				if e.Attrs == nil {
					e.Attrs = map[string]object.Value{}
				}
				e.Attrs[name] = kw.Values()[i]
			}
		}
		if !known && len(e.Cls.Fields) > 0 {
			return ctx.Raise(object.ArgumentError, "%s.new: unknown field %s", e.Cls.QualifiedName(), name)
		}
	}
	return nil
}

// installExceptionAttrs gives the exceptions of Ruby's core that carry more
// than a message their initialize and readers.
func installExceptionAttrs() {
	exc := func(self object.Value) *object.Exception { return self.Exception() }
	// reader reads an attribute, which is def when the exception has none,
	// or raises an ArgumentError when there is no default either.
	reader := func(cls *object.Class, name string, def object.Value) {
		cls.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if v, ok := exc(self).Attr(name); ok {
				return v, nil
			}
			if def.IsZero() {
				return object.Value{}, ctx.Raise(object.ArgumentError, "no %s is available", name)
			}
			return def, nil
		})
	}
	set := func(e *object.Exception, name string, v object.Value) {
		if e.Attrs == nil {
			e.Attrs = map[string]object.Value{}
		}
		e.Attrs[name] = v
	}

	// NameError#initialize(msg = nil, name = nil, receiver: nil), and
	// NoMethodError's with args after the name.
	def(object.NameError, "initialize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		e, err := building(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		switch {
		case e.Cls.IsA(object.NoMethodError) && len(args) > 3:
			return object.Value{}, argError(ctx, len(args), "0..3")
		case !e.Cls.IsA(object.NoMethodError) && len(args) > 2:
			return object.Value{}, argError(ctx, len(args), "0..2")
		}
		if e.Message, err = message(ctx, e.Cls, args, 0); err != nil {
			return object.Value{}, err
		}
		if len(args) > 1 {
			set(e, "name", args[1])
		}
		if len(args) > 2 {
			set(e, "args", args[2])
		}
		return object.Nil, keywords(ctx, e, kw, "receiver")
	})
	reader(object.NameError, "name", object.Nil)
	reader(object.NameError, "receiver", object.Value{})
	reader(object.NoMethodError, "args", object.NewList())

	// KeyError#initialize(msg = nil, receiver: nil, key: nil)
	def(object.KeyError, "initialize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		e, err := building(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if e.Message, err = message(ctx, e.Cls, args, 0); err != nil {
			return object.Value{}, err
		}
		return object.Nil, keywords(ctx, e, kw, "receiver", "key")
	})
	reader(object.KeyError, "key", object.Value{})
	reader(object.KeyError, "receiver", object.Value{})

	// SystemExit#initialize(status = 0, msg = "SystemExit"): the status may
	// be true for 0 or false for 1, and may be left out before the message.
	def(object.SystemExit, "initialize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		e, err := building(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		if len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "0..2")
		}
		status := int64(0)
		if len(args) > 0 {
			if s, ok := exitStatus(args[0]); ok {
				status = s
				args = args[1:]
			} else if len(args) == 2 {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", args[0].Class().QualifiedName())
			}
		}
		if e.Message, err = message(ctx, e.Cls, args, 0); err != nil {
			return object.Value{}, err
		}
		set(e, "status", object.Int(status))
		return object.Nil, keywords(ctx, e, kw)
	})
	reader(object.SystemExit, "status", object.Int(0))
	def(object.SystemExit, "success?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := exc(self).Attr("status")
		return object.Bool(!ok || v.Kind() == object.KindInt && v.Int() == 0), nil
	})
	// throw makes an UncaughtThrowError with its tag and value.
	reader(object.UncaughtThrowError, "tag", object.Nil)
	reader(object.UncaughtThrowError, "value", object.Nil)
}

// exitStatus reads the status exit and SystemExit.new take: an Integer, or
// true for 0 and false for 1.
func exitStatus(v object.Value) (int64, bool) {
	switch v.Kind() {
	case object.KindInt:
		return int64(v.Int()), true
	case object.KindBool:
		if v.Truthy() {
			return 0, true
		}
		return 1, true
	}
	return 0, false
}
