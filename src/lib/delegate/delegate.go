// Package delegate is Ruby's delegate library, built into cameo:
// Delegator and SimpleDelegator, which forward every method they lack to
// the object they wrap, and DelegateClass, a class that does the same for
// a given class's instances.
//
// It follows delegate 0.4.0, the version Ruby 3.4 ships. A delegator is a
// value, so __setobj__ returns the delegator with the new object, for the
// variable to take, where Ruby changes it in place. The wrapped object
// counts in a delegator's identity as a Hash key, as its fields do.
package delegate

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	delegatorClass = object.NewClass("Delegator", object.ObjectClass)
	simpleClass    = object.NewClass("SimpleDelegator", delegatorClass)
)

// target is what a delegator wraps, kept on the instance; it counts in
// the delegator's identity as a key.
type target struct{ obj object.Value }

func (t *target) KeyValue() object.Value { return t.obj }

func targetOf(ctx object.Context, self object.Value) (object.Value, error) {
	inst, ok := self.AsInstance()
	if !ok {
		return object.Value{}, ctx.Raise(object.TypeError, "not a delegator")
	}
	t, ok := inst.Message.(*target)
	if !ok {
		return object.Value{}, ctx.Raise(object.ArgumentError, "not delegated")
	}
	return t.obj, nil
}

// build makes an instance of cls around obj: the class's own initialize
// runs when it has one, and super reaches Delegator's.
func build(ctx object.Context, cls *object.Class, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	// The fields start as the core starts them: a default, or empty.
	inst := &object.Instance{Cls: cls, Fields: make([]object.Value, len(cls.Fields))}
	for i, f := range cls.Fields {
		inst.Fields[i] = f.Default
		if f.Default.IsZero() {
			inst.Fields[i] = object.Empty(f.Type)
		}
	}
	v := object.InstanceValue(inst)
	if m := cls.Lookup("initialize"); m != nil && m.Body != nil {
		if _, err := ctx.CallWith(v, "initialize", args, kw, block); err != nil {
			return object.Value{}, err
		}
		if _, ok := inst.Message.(*target); !ok {
			return object.Value{}, ctx.Raise(object.ArgumentError, "%s's initialize did not call super with the object to wrap", cls.QualifiedName())
		}
		return v, nil
	}
	if len(args) != 1 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
	}
	inst.Message = &target{args[0]}
	return v, nil
}

// Install defines Delegator, SimpleDelegator and DelegateClass.
// lib.Install calls it once.
func Install() {
	d := delegatorClass
	object.DefineNative("Delegator", object.ClassValue(d))
	object.DefineNative("SimpleDelegator", object.ClassValue(simpleClass))
	d.Consts["VERSION"] = object.String("0.4.0")
	d.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		if cls == delegatorClass {
			return object.Value{}, ctx.Raise(object.TypeError, "Delegator is abstract: use SimpleDelegator or DelegateClass")
		}
		return build(ctx, cls, args, kw, block)
	})
	d.Define("initialize", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) != 1 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1")
		}
		self.Instance().Message = &target{args[0]}
		return object.Nil, nil
	})
	d.Define("__getobj__", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return targetOf(ctx, self)
	})
	d.Define("__setobj__", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst := self.Instance()
		return object.InstanceValue(&object.Instance{Cls: inst.Cls, Fields: inst.Fields, Message: &target{args[0]}}), nil
	})
	// answers says whether the wrapped object answers a method, through
	// its own respond_to?, so a target that answers through method_missing,
	// another delegator for one, is forwarded to as Ruby's Delegator does.
	answers := func(ctx object.Context, obj object.Value, name object.Value) (bool, error) {
		v, err := ctx.Call(obj, "respond_to?", name)
		if err != nil {
			return false, err
		}
		return v.Truthy(), nil
	}
	d.Define("method_missing", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) == 0 {
			return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
		}
		obj, err := targetOf(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		name, _ := args[0].AsSymbol()
		ok, err := answers(ctx, obj, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if !ok {
			return object.Value{}, &object.Exception{Cls: object.NoMethodError,
				Message: "undefined method '" + name + "' for an instance of " + self.Class().QualifiedName(),
				Attrs:   map[string]object.Value{"name": args[0], "receiver": self, "args": object.NewList(args[1:]...)}}
		}
		return ctx.CallWith(obj, name, args[1:], kw, block)
	})
	d.Define("respond_to_missing?", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		obj, err := targetOf(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		ok, err := answers(ctx, obj, args[0])
		return object.Bool(ok), err
	})
	// What Object defines is forwarded by name, as Delegator does; the
	// rest reaches the wrapped object through method_missing, so
	// respond_to? answers for the object. freeze returns the delegator, as
	// Ruby's does.
	forward := func(name string) {
		d.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			obj, err := targetOf(ctx, self)
			if err != nil {
				return object.Value{}, err
			}
			return ctx.CallWith(obj, name, args, kw, block)
		})
	}
	for _, name := range []string{"to_s", "inspect", "eql?", "hash", "<=>", "===", "=~", "frozen?", "!"} {
		forward(name)
	}
	d.Define("freeze", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	d.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if isDelegator(args[0]) {
			other, err := targetOf(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			args = []object.Value{other}
		}
		obj, err := targetOf(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		eq, err := ctx.Equal(obj, args[0])
		return object.Bool(eq), err
	})
	d.Define("!=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		eq, err := ctx.Call(self, "==", args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(!eq.Truthy()), nil
	})

	// DelegateClass(klass) is a new class of delegators to klass's
	// instances; what it forwards is what the wrapped object answers.
	object.ObjectClass.Define("DelegateClass", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "DelegateClass takes a class, not %s", stdlib.KindName(args[0]))
		}
		anon := object.NewClass("", delegatorClass)
		anon.Consts["DELEGATED_CLASS"] = object.ClassValue(cls)
		return object.ClassValue(anon), nil
	})
}

func isDelegator(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(delegatorClass)
}
