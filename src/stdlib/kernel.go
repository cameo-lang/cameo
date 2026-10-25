package stdlib

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
)

// installKernel defines puts, print, p, warn, raise and format on every
// value, as Ruby's Kernel does.
func installKernel() {
	o := object.ObjectClass
	def(o, "puts", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, puts(ctx, false, args)
	})
	def(o, "print", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.ToS(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(false, s); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	def(o, "p", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.Inspect(a)
			if err != nil {
				return object.Value{}, err
			}
			if err := ctx.Write(false, s+"\n"); err != nil {
				return object.Value{}, err
			}
		}
		switch len(args) {
		case 0:
			return object.Nil, nil
		case 1:
			return args[0], nil
		}
		return object.NewList(args...), nil
	})
	def(o, "pp", -1, o.Methods["p"].Native)
	def(o, "warn", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, puts(ctx, true, args)
	})
	def(o, "raise", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, raise(ctx, args)
	})
	def(o, "fail", -1, o.Methods["raise"].Native)
	// exit raises SystemExit, which ensure clauses see and a rescue may
	// catch; the command ends with its status when nothing does.
	def(o, "exit", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		status := int64(0)
		if len(args) == 1 {
			s, ok := exitStatus(args[0])
			if !ok {
				return object.Value{}, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", args[0].Class().QualifiedName())
			}
			status = s
		}
		return object.Value{}, systemExit(status, "exit")
	})
	// abort writes its message to standard error and raises SystemExit
	// with status 1.
	def(o, "abort", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		if len(args) == 0 {
			return object.Value{}, systemExit(1, "exit")
		}
		msg, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if err := ctx.Write(true, msg+"\n"); err != nil {
			return object.Value{}, err
		}
		return object.Value{}, systemExit(1, msg)
	})
	def(o, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.Value{}, argError(ctx, 0, "1+")
		}
		f, err := wantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		s, err := format(ctx, f, args[1:])
		return object.String(s), err
	})
	def(o, "sprintf", -1, o.Methods["format"].Native)
	def(o, "loop", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "loop"); err != nil {
			return object.Value{}, err
		}
		for {
			if _, err := ctx.Yield(block); err != nil {
				if exc, ok := err.(*object.Exception); ok && exc.Cls.IsA(object.StopIteration) {
					return object.Nil, nil
				}
				return object.Value{}, err
			}
		}
	})
	def(o, "block_given?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// The evaluator handles this for user methods; a native call site has
		// no enclosing method.
		return object.False, nil
	})
	def(o, "Integer", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch args[0].Kind() {
		case object.KindInt, object.KindBig:
			return args[0], nil
		case object.KindFloat:
			return floatToInteger(ctx, args[0].Float())
		case object.KindString, object.KindBytes, object.KindASCII:
			v := args[0].Str()
			i, ok := parseInteger(strings.TrimSpace(v))
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "invalid value for Integer(): %s", object.QuoteString(v))
			}
			return i, nil
		}
		return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Integer", kindName(args[0]))
	})
	def(o, "Float", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return toFloat(ctx, args[0])
	})
	def(o, "String", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(args[0])
		return object.String(s), err
	})
	def(o, "Array", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsNil() {
			return object.NewList(), nil
		}
		elems, err := toList(ctx, args[0])
		if err != nil {
			return object.NewList(args[0]), nil
		}
		return object.NewList(append([]object.Value{}, elems...)...), nil
	})
}

// puts writes each argument on its own line, a list element by element,
// and a bare newline for no arguments.
func puts(ctx object.Context, stderr bool, args []object.Value) error {
	if len(args) == 0 {
		return ctx.Write(stderr, "\n")
	}
	for _, a := range args {
		if l, ok := a.AsList(); ok {
			if len(l.Elems) == 0 {
				if err := ctx.Write(stderr, "\n"); err != nil {
					return err
				}
			}
			if err := puts(ctx, stderr, l.Elems); err != nil {
				return err
			}
			continue
		}
		s, err := ctx.ToS(a)
		if err != nil {
			return err
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if err := ctx.Write(stderr, s); err != nil {
			return err
		}
	}
	return nil
}

// raise builds the exception for raise's arguments: a message makes a
// RuntimeError, a class is instantiated with an optional message, an
// exception is raised as it is.
func raise(ctx object.Context, args []object.Value) error {
	if len(args) == 0 {
		return &object.Exception{Cls: object.RuntimeError, Message: "unhandled exception"}
	}
	switch args[0].Kind() {
	case object.KindString, object.KindBytes, object.KindASCII:
		v := args[0].Str()
		if len(args) > 1 {
			return argError(ctx, len(args), "1")
		}
		return &object.Exception{Cls: object.RuntimeError, Message: string(v)}
	case object.KindException:
		v := args[0].Exception()
		if len(args) > 1 {
			if msg, err := wantString(ctx, args[1]); err == nil {
				out := v.Copy()
				out.Message, out.Line = msg, 0
				return out
			}
		}
		return v
	case object.KindClass:
		v := args[0].Cls()
		if !v.IsA(object.ExceptionClass) {
			return ctx.Raise(object.TypeError, "exception class/object expected")
		}
		if !plainException(v) {
			out, err := ctx.CallWith(args[0], "exception", args[1:min(len(args), 2)], nil, object.Value{})
			if err != nil {
				return err
			}
			exc, ok := out.AsException()
			if !ok {
				return ctx.Raise(object.TypeError, "exception object expected")
			}
			return exc
		}
		msg := v.QualifiedName()
		if len(args) > 1 {
			s, err := ctx.ToS(args[1])
			if err != nil {
				return err
			}
			msg = s
		}
		return &object.Exception{Cls: v, Message: msg}
	}
	return ctx.Raise(object.TypeError, "exception class/object expected")
}

// plainException reports whether an exception of cls is made here rather
// than by cls.exception: its class declares no fields and has Exception's
// own initialize, exception and new. It reads the classes' own methods rather than looking
// initialize up, which costs a raise in a loop a tenth of its time.
func plainException(cls *object.Class) bool {
	if len(cls.Fields) > 0 {
		return false
	}
	for c := cls; c != nil && c != object.ExceptionClass; c = c.Super {
		if c.Methods["initialize"] != nil || len(c.Includes) > 0 || c.SingletonMethods["exception"] != nil || c.SingletonMethods["new"] != nil {
			return false
		}
	}
	return true
}

// systemExit makes the SystemExit for exit and abort.
func systemExit(status int64, msg string) error {
	return &object.Exception{Cls: object.SystemExit, Message: msg, Attrs: map[string]object.Value{"status": object.Int(status)}}
}

// toFloat is Kernel#Float of one value: a number as a float, or a string
// parsed as one.
func toFloat(ctx object.Context, v object.Value) (object.Value, error) {
	switch v.Kind() {
	case object.KindInt, object.KindBig:
		f, _ := v.AsNumber()
		return object.Float(f), nil
	case object.KindFloat:
		return v, nil
	case object.KindString, object.KindBytes, object.KindASCII:
		s := v.Str()
		f, ok := parseFloat(strings.TrimSpace(string(s)))
		if !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid value for Float(): %s", object.QuoteString(string(s)))
		}
		return object.Float(f), nil
	}
	return object.Value{}, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(v))
}
