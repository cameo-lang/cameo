package stdlib

import (
	"strings"

	"github.com/radiospiel/amber/src/object"
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
				return nil, err
			}
			if err := ctx.Write(false, s); err != nil {
				return nil, err
			}
		}
		return object.Nil, nil
	})
	def(o, "p", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, a := range args {
			s, err := ctx.Inspect(a)
			if err != nil {
				return nil, err
			}
			if err := ctx.Write(false, s+"\n"); err != nil {
				return nil, err
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
		return nil, raise(ctx, args)
	})
	def(o, "fail", -1, o.Methods["raise"].Native)
	def(o, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return nil, argError(ctx, 0, "1+")
		}
		f, err := wantString(ctx, args[0])
		if err != nil {
			return nil, err
		}
		s, err := format(ctx, f, args[1:])
		return object.String(s), err
	})
	def(o, "sprintf", -1, o.Methods["format"].Native)
	def(o, "loop", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if err := needBlock(ctx, block, "loop"); err != nil {
			return nil, err
		}
		for {
			if _, err := ctx.Yield(block); err != nil {
				if exc, ok := err.(*object.Exception); ok && exc.Cls.IsA(object.StopIteration) {
					return object.Nil, nil
				}
				return nil, err
			}
		}
	})
	def(o, "block_given?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// The evaluator handles this for user methods; a native call site has
		// no enclosing method.
		return object.False, nil
	})
	def(o, "Integer", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch v := args[0].(type) {
		case object.Int:
			return v, nil
		case object.Float:
			return object.IntValue(int64(v)), nil
		case object.String:
			i, ok := parseInt(strings.TrimSpace(string(v)))
			if !ok {
				return nil, ctx.Raise(object.ArgumentError, "invalid value for Integer(): %s", object.QuoteString(string(v)))
			}
			return object.IntValue(i), nil
		}
		return nil, ctx.Raise(object.TypeError, "can't convert %s into Integer", kindName(args[0]))
	})
	def(o, "Float", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch v := args[0].(type) {
		case object.Int:
			return object.Float(v), nil
		case object.Float:
			return v, nil
		case object.String:
			f, ok := parseFloat(strings.TrimSpace(string(v)))
			if !ok {
				return nil, ctx.Raise(object.ArgumentError, "invalid value for Float(): %s", object.QuoteString(string(v)))
			}
			return object.Float(f), nil
		}
		return nil, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(args[0]))
	})
	def(o, "String", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(args[0])
		return object.String(s), err
	})
	def(o, "Array", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0] == object.Nil {
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
		if l, ok := a.(*object.List); ok {
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
	switch v := args[0].(type) {
	case object.String:
		if len(args) > 1 {
			return argError(ctx, len(args), "1")
		}
		return &object.Exception{Cls: object.RuntimeError, Message: string(v)}
	case *object.Exception:
		if len(args) > 1 {
			if msg, err := wantString(ctx, args[1]); err == nil {
				return &object.Exception{Cls: v.Cls, Message: msg}
			}
		}
		return v
	case *object.Class:
		if !v.IsA(object.ExceptionClass) {
			return ctx.Raise(object.TypeError, "exception class/object expected")
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
