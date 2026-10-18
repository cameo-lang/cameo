package eval

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
	"path/filepath"
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
	// A symbol's proc calls the method of that name on its first argument,
	// as &:name does.
	object.SymbolClass.Define("to_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return procValue(&Proc{Sym: self.Str()}), nil
	})
	// A hash's proc is a lambda that looks a key up, as Ruby's
	// Hash#to_proc is.
	object.MapClass.Define("to_proc", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return hashProc(self), nil
	})
	procClass.Define("curry", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NotImplementedErr, "curry is not supported")
	})
}

func init() {
	object.ObjectClass.Define("with", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).withChanges(self, args, kw, block)
	})
	// tap is Ruby's: it yields the receiver and returns it unchanged.
	object.ObjectClass.Define("tap", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.LocalJumpError, "no block given (yield)")
		}
		if _, err := ctx.Yield(block, self); err != nil {
			return object.Value{}, err
		}
		return self, nil
	})
	// Class#new builds an instance of a user class and runs its initialize.
	object.ClassClass.Define("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).newInstance(self.Cls(), args, kw, block)
	})
	object.ObjectClass.Define("require", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.(*Interp).require(args[0])
	})
	// method_missing is what super reaches from a class's own: the
	// NoMethodError for the name.
	object.ObjectClass.Define("method_missing", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := ctx.(*Interp)
		if len(args) == 0 {
			return object.Value{}, in.raise(object.ArgumentError, 0, "no method name given")
		}
		name, _ := args[0].AsSymbol()
		return object.Value{}, in.raise(object.NoMethodError, 0, "undefined method '%s' for %s", name, typeName(self))
	})
	// __FILE__ is the script's path as it was given, and __dir__ the
	// absolute directory it is in.
	object.ObjectClass.Define("__FILE__", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(ctx.(*Interp).file), nil
	})
	object.ObjectClass.Define("__dir__", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		abs, err := filepath.Abs(ctx.(*Interp).file)
		if err != nil {
			return object.Nil, nil
		}
		return object.String(filepath.Dir(abs)), nil
	})
	object.ClassClass.Define("allocate", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, ctx.Raise(object.NoMethodError, "allocate is not supported: use new")
	})
}

// newInstance is Class#new: an instance with its fields at their defaults,
// filled by initialize. Without initialize the fields come as keywords.
func (in *Interp) newInstance(cls *object.Class, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if cls == object.ClassClass {
		return in.newClass(args, block)
	}
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
			case object.KindString, object.KindBytes, object.KindASCII:
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

// hashProc is the proc of a hash: a lambda that returns hash[key].
func hashProc(h object.Value) object.Value {
	return procValue(&Proc{Sym: "[]", Recv: h, Strict: true})
}

// builtinLibraries are what require loads without a file: libraries
// cameo builds in, each with the ones it loads along with it.
var builtinLibraries = map[string][]string{
	"set":        nil,
	"time":       nil,
	"pp":         nil,
	"pathname":   nil,
	"cgi":        {"cgi/escape", "cgi/util"},
	"cgi/escape": nil,
	"cgi/util":   {"cgi/escape"},
}

// require loads a library cameo builds in, true the first time and false
// after, as Ruby's does; any other name is a LoadError, since a script is
// one file.
func (in *Interp) require(name object.Value) (object.Value, error) {
	s, ok := name.AsString()
	if !ok {
		return object.Value{}, in.raise(object.TypeError, 0, "no implicit conversion of %s into String", typeName(name))
	}
	also, builtin := builtinLibraries[s]
	if !builtin {
		return object.Value{}, in.raise(object.LoadError, 0, "cannot load such file -- %s", s)
	}
	if in.loaded[s] {
		return object.False, nil
	}
	if in.loaded == nil {
		in.loaded = map[string]bool{}
	}
	in.loaded[s] = true
	for _, lib := range also {
		in.loaded[lib] = true
	}
	return object.True, nil
}
