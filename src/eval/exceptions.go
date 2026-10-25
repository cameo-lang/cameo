package eval

import (
	"github.com/cameo-lang/cameo/src/object"
)

// Exceptions: Exception.new, which runs a class's initialize on the
// exception it makes, and the cause an exception gets from the one being
// rescued when it is raised. The design is in doc/design/exceptions.md.

// newException is Exception.new: an exception with the class's name for
// its message and its fields at their defaults, which initialize sets, the
// class's own or Exception's. A class without an initialize of its own
// takes its fields as keywords, and the message and the rest of what
// Ruby's exceptions take as Exception's initialize does.
func (in *Interp) newException(cls *object.Class, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	exc := &object.Exception{Cls: cls, Message: cls.QualifiedName(), Fields: defaultFields(cls)}
	m := cls.Lookup("initialize")
	self := object.ExceptionValue(exc)
	exc.Building = true
	defer func() { exc.Building = false }()
	if m.Body != nil {
		if _, err := in.invoke(m, self, args, kw, block, 0); err != nil {
			return object.Value{}, err
		}
		return self, nil
	}
	if len(cls.Fields) > 0 {
		// One positional map stands for the keywords: what a Go host passes.
		if len(args) == 1 && kw == nil {
			if m, ok := args[0].AsMap(); ok {
				kw, args = m, nil
			}
		}
		if kw != nil {
			rest := object.NewMapBuilder()
			others := false
			for i, k := range kw.Keys() {
				name, ok := k.AsFieldName()
				if !ok {
					return object.Value{}, in.raise(object.ArgumentError, 0, "%s.new: fields are named by symbols", cls.QualifiedName())
				}
				idx, ok := cls.FieldIndex[name]
				if !ok {
					rest.Set(k, kw.Values()[i])
					others = true
					continue
				}
				v, err := in.checkField(cls.Fields[idx], kw.Values()[i], 0)
				if err != nil {
					return object.Value{}, err
				}
				exc.Fields[idx] = v
			}
			kw = nil
			if others {
				kw = rest.Map()
			}
		}
	}
	if _, err := m.Native(in, self, args, kw, block); err != nil {
		return object.Value{}, err
	}
	return self, nil
}

// defaultFields returns the fields of a new exception of cls at their
// defaults, nil for a class that declares none.
func defaultFields(cls *object.Class) []object.Value {
	if len(cls.Fields) == 0 {
		return nil
	}
	fields := make([]object.Value, len(cls.Fields))
	for i, f := range cls.Fields {
		fields[i] = f.Default
		if f.Default.IsZero() {
			fields[i] = emptyFor(f.Type)
		}
	}
	return fields
}

// installRaise wraps Kernel's raise and fail: a raise without arguments
// in a rescue clause raises the exception it rescues again, and `cause:`
// gives the exception its cause, nil for none.
func installRaise() {
	raise := object.ObjectClass.Methods["raise"].Native
	wrapped := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := ctx.(*Interp)
		var cause object.Value
		if kw != nil {
			if c, ok := kw.Get(object.Symbol("cause")); ok {
				if !c.IsNil() && c.Kind() != object.KindException {
					return object.Value{}, in.raise(object.TypeError, 0, "exception object expected")
				}
				cause = c
			}
		}
		if len(args) == 0 {
			if !cause.IsZero() {
				return object.Value{}, in.raise(object.ArgumentError, 0, "only cause is given with no arguments")
			}
			if in.rescuing != nil {
				return object.Value{}, in.rescuing
			}
		}
		_, err := raise(ctx, self, args, nil, block)
		exc, ok := err.(*object.Exception)
		if !ok || cause.IsZero() {
			return object.Value{}, err
		}
		if c, ok := cause.AsException(); ok && c == exc {
			return object.Value{}, in.raise(object.ArgumentError, 0, "circular causes")
		}
		exc = exc.Copy()
		exc.Cause = cause
		return object.Value{}, exc
	}
	object.ObjectClass.Define("raise", -1, wrapped)
	object.ObjectClass.Define("fail", -1, wrapped)
}

// caused returns err with the rescued exception as its cause, when err is
// another exception that was raised without one: what Ruby gives an
// exception raised while $! is set. An exception whose causes reach err
// already keeps none, so a cause never leads back to itself.
func caused(err error, rescued *object.Exception) error {
	exc, ok := err.(*object.Exception)
	if !ok || exc == rescued || !exc.Cause.IsZero() {
		return err
	}
	for c := rescued; c != nil; {
		if c == exc {
			return err
		}
		next, ok := c.Cause.AsException()
		if !ok {
			break
		}
		c = next
	}
	out := exc.Copy()
	out.Cause = object.ExceptionValue(rescued)
	return out
}
