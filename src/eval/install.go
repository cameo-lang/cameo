package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/stdlib"
)

// installStdlib installs the standard library once per process: the Go
// methods of the builtin classes, and the Proc methods, which know this
// package's Proc.
func installStdlib() {
	stdlib.Install()
	procClass := object.ProcClass
	call := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).callBlock(self, args, kw, block)
	}
	procClass.Define("call", -1, call)
	procClass.Define("()", -1, call)
	procClass.Define("[]", -1, call)
	procClass.Define("yield", -1, call)
	procClass.Define("===", -1, call)
	procClass.Define("to_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	procClass.Define("lambda?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(proc(self).Strict), nil
	})
	procClass.Define("arity", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(proc(self).Arity()), nil
	})
	procClass.Define("curry", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "curry is not supported")
	})
}

func init() {
	object.ObjectClass.Define("tap", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).tap(self, block)
	})
	// Class#new builds an instance of a user class and runs its initialize.
	object.ClassClass.Define("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).newInstance(self.Cls(), args, kw, block)
	})
	object.ClassClass.Define("allocate", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NoMethodError, "allocate is not supported: use new")
	})
}

// newInstance is Class#new: an instance with its fields at their defaults,
// filled by initialize. Without initialize the fields come as keywords.
func (in *Interp) newInstance(cls *object.Class, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if cls.IsModule {
		return object.Value{}, in.raise(object.NoMethodError, 0, "undefined method 'new' for module %s", cls.QualifiedName())
	}
	if cls.Digest == "" && cls != object.ObjectClass {
		return object.Value{}, in.raise(object.NoMethodError, 0, "undefined method 'new' for class %s: use a literal", cls.QualifiedName())
	}
	inst := &object.Instance{Cls: cls, Fields: make([]object.Value, len(cls.Fields))}
	for i, f := range cls.Fields {
		inst.Fields[i] = f.Default
		if f.Default.IsZero() {
			inst.Fields[i] = emptyFor(f.Type)
		}
	}
	if m := cls.Lookup("initialize"); m != nil && m.Body != nil {
		if _, err := in.invoke(m, object.InstanceValue(inst), args, kw, block, 0); err != nil {
			return object.Value{}, err
		}
		return object.InstanceValue(inst), nil
	}
	// One positional map stands for the keywords: what a Go host passes.
	if len(args) == 1 && kw == nil {
		if m, ok := args[0].AsMap(); ok {
			kw, args = m, nil
		}
	}
	if len(args) > 0 {
		return object.Value{}, in.raise(object.ArgumentError, 0, "wrong number of arguments (given %d, expected 0): %s takes its fields as keywords", len(args), cls.QualifiedName())
	}
	if kw != nil {
		for i, k := range kw.Keys() {
			var name string
			switch k.Kind() {
			case object.KindSymbol:
				k := k.Str()
				name = string(k)
			case object.KindString:
				k := k.Str()
				name = string(k)
			default:
				return object.Value{}, in.raise(object.ArgumentError, 0, "%s.new: fields are named by symbols", cls.QualifiedName())
			}
			idx, ok := cls.FieldIndex[name]
			if !ok {
				return object.Value{}, in.raise(object.ArgumentError, 0, "%s.new: unknown field %s", cls.QualifiedName(), name)
			}
			v, err := in.checkField(cls.Fields[idx], kw.Values()[i], 0)
			if err != nil {
				return object.Value{}, err
			}
			inst.Fields[idx] = v
		}
	}
	return object.InstanceValue(inst), nil
}
