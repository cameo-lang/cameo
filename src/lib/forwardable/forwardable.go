// Package forwardable is Ruby's forwardable library, built into cameo:
// extend Forwardable in a class body, and def_delegator, def_delegators,
// def_instance_delegator(s) and delegate define methods that forward to
// an accessor's value; SingleForwardable does the same for one object, a
// class or a module.
//
// It follows forwardable 1.3.3, the version Ruby 3.4 ships. The accessor
// is a method name, :items, or an instance variable's, :@items, which
// names the field here; Ruby also takes an expression as a String, which
// cameo does not evaluate.
package forwardable

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module = object.NewModule("Forwardable")
	single = object.NewModule("SingleForwardable")
)

// accessorName reads an accessor: a Symbol or String naming a method, or
// an instance variable, whose name without @ is a field's and its reader.
func accessorName(ctx object.Context, v object.Value) (string, error) {
	name, ok := v.AsSymbol()
	if !ok {
		s, err := stdlib.WantString(ctx, v)
		if err != nil {
			return "", err
		}
		name = s
	}
	if strings.ContainsAny(name, ".()[] ") {
		return "", ctx.Raise(object.ArgumentError, "the accessor %q is an expression, which cameo does not evaluate: name a method or a field", name)
	}
	return strings.TrimPrefix(name, "@"), nil
}

func methodName(ctx object.Context, v object.Value) (string, error) {
	if name, ok := v.AsSymbol(); ok {
		return name, nil
	}
	return stdlib.WantString(ctx, v)
}

// forwarder is the method defined: it calls method on the accessor's
// value with the arguments and the block.
func forwarder(accessor, method string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		target, err := ctx.Call(self, accessor)
		if err != nil {
			return object.Value{}, err
		}
		return ctx.CallWith(target, method, args, kw, block)
	}
}

// define installs a forwarder as an instance method of a class, or a
// singleton method of any object.
func define(ctx object.Context, self object.Value, instance bool, accessor object.Value, method object.Value, alias object.Value) error {
	acc, err := accessorName(ctx, accessor)
	if err != nil {
		return err
	}
	meth, err := methodName(ctx, method)
	if err != nil {
		return err
	}
	name := meth
	if !alias.IsZero() && !alias.IsNil() {
		if name, err = methodName(ctx, alias); err != nil {
			return err
		}
	}
	cls, ok := self.AsClass()
	if instance {
		if !ok {
			return ctx.Raise(object.TypeError, "def_delegator defines an instance method: extend Forwardable in a class")
		}
		cls.Define(name, -1, forwarder(acc, meth))
		return nil
	}
	if !ok {
		return ctx.Raise(object.TypeError, "SingleForwardable forwards from a class or a module here, not from %s", stdlib.KindName(self))
	}
	cls.DefineSingleton(name, -1, forwarder(acc, meth))
	return nil
}

// pairs reads delegate's argument: a Hash of method or methods => accessor.
func pairs(ctx object.Context, self object.Value, instance bool, args []object.Value, kw *object.Map) error {
	var maps []*object.Map
	for _, a := range args {
		m, ok := a.AsMap()
		if !ok {
			return ctx.Raise(object.ArgumentError, "delegate takes method => accessor pairs")
		}
		maps = append(maps, m)
	}
	if kw != nil {
		maps = append(maps, kw)
	}
	for _, m := range maps {
		for i, k := range m.Keys() {
			accessor := m.Values()[i]
			methods := []object.Value{k}
			if list, ok := k.AsList(); ok {
				methods = list.Elems
			}
			for _, meth := range methods {
				if err := define(ctx, self, instance, accessor, meth, object.Value{}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Install defines Forwardable and SingleForwardable. lib.Install calls it
// once.
func Install() {
	object.DefineNative("Forwardable", object.ClassValue(module))
	object.DefineNative("SingleForwardable", object.ClassValue(single))
	module.Consts["VERSION"] = object.NewString("1.3.3", object.ASCII)
	module.Consts["FORWARDABLE_VERSION"] = object.NewString("1.3.3", object.ASCII)
	one := func(instance bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 2 || len(args) > 3 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "2..3")
			}
			alias := object.Value{}
			if len(args) == 3 {
				alias = args[2]
			}
			return object.Nil, define(ctx, self, instance, args[0], args[1], alias)
		}
	}
	many := func(instance bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 2 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "2+")
			}
			for _, m := range args[1:] {
				if name, _ := m.AsSymbol(); name == "__send__" || name == "__id__" {
					continue
				}
				if err := define(ctx, self, instance, args[0], m, object.Value{}); err != nil {
					return object.Value{}, err
				}
			}
			return object.Nil, nil
		}
	}
	hash := func(instance bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Nil, pairs(ctx, self, instance, args, kw)
		}
	}
	for _, name := range []string{"def_delegator", "def_instance_delegator"} {
		module.Define(name, -1, one(true))
	}
	for _, name := range []string{"def_delegators", "def_instance_delegators"} {
		module.Define(name, -1, many(true))
	}
	for _, name := range []string{"delegate", "instance_delegate"} {
		module.Define(name, -1, hash(true))
	}
	for _, name := range []string{"def_delegator", "def_single_delegator"} {
		single.Define(name, -1, one(false))
	}
	for _, name := range []string{"def_delegators", "def_single_delegators"} {
		single.Define(name, -1, many(false))
	}
	for _, name := range []string{"delegate", "single_delegate"} {
		single.Define(name, -1, hash(false))
	}
}
