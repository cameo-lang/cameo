package stdlib

import (
	"maps"
	"slices"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// installClass defines the methods of classes and modules themselves.
func installClass() {
	m := object.ModuleClass
	cls := func(self object.Value) *object.Class { return self.Cls() }
	def(m, "name", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(cls(self).QualifiedName()), nil
	})
	def(m, "to_s", 0, m.Methods["name"].Native)
	def(m, "inspect", 0, m.Methods["name"].Native)
	def(m, "===", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(args[0].Class().IsA(cls(self))), nil
	})
	def(m, "==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(object.Same(self, args[0])), nil
	})
	// singleton_class? is false for every class: cameo has no singleton
	// classes, and the classes of nil, true and false are not ones in Ruby.
	def(m, "singleton_class?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.False, nil
	})
	def(m, "ancestors", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		anc := cls(self).Ancestors()
		out := make([]object.Value, len(anc))
		for i, a := range anc {
			out[i] = object.ClassValue(a)
		}
		return object.NewList(out...), nil
	})
	def(m, "include?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, ok := args[0].AsClass()
		if !ok || !other.IsModule {
			return object.Value{}, ctx.Raise(object.TypeError, "wrong argument type %s (expected Module)", args[0].Class().QualifiedName())
		}
		return object.Bool(cls(self) != other && cls(self).IsA(other)), nil
	})
	def(m, "<", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, ok := args[0].AsClass()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "compared with non class/module")
		}
		if cls(self) == other {
			return object.False, nil
		}
		if cls(self).IsA(other) {
			return object.True, nil
		}
		if other.IsA(cls(self)) {
			return object.False, nil
		}
		return object.Nil, nil
	})
	def(m, "<=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, ok := args[0].AsClass()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "compared with non class/module")
		}
		if cls(self).IsA(other) {
			return object.True, nil
		}
		if other.IsA(cls(self)) {
			return object.False, nil
		}
		return object.Nil, nil
	})
	// instance_methods(inherited = true) are the methods a value of the
	// class answers, without the ones Ruby keeps private; with false, the
	// class's own only.
	def(m, "instance_methods", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, ArgError(ctx, len(args), "0..1")
		}
		return methodSymbols(instanceMethodNames(ctx, cls(self), len(args) == 0 || args[0].Truthy())), nil
	})
	// singleton_methods(all = true) are the class methods: the class's own,
	// its superclasses' and those of the modules they extend; with false,
	// the class's own only.
	def(m, "singleton_methods", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, ArgError(ctx, len(args), "0..1")
		}
		return methodSymbols(singletonMethodNames(ctx, cls(self), len(args) == 0 || args[0].Truthy())), nil
	})
	def(m, "method_defined?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		m := cls(self).Lookup(name)
		return object.Bool(m != nil && !m.Private), nil
	})
	def(m, "const_get", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		for _, a := range cls(self).Ancestors() {
			if v, ok := a.Consts[name]; ok {
				return v, nil
			}
		}
		return object.Value{}, ctx.Raise(object.NameError, "uninitialized constant %s::%s", cls(self).QualifiedName(), name)
	})
	def(m, "constants", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		names := []object.Value{}
		for n := range cls(self).Consts {
			names = append(names, object.Symbol(n))
		}
		return object.NewList(names...), nil
	})
	def(m, "hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(hashString(object.Key(self))), nil
	})
	k := object.ClassClass
	def(k, "superclass", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if cls(self).Super == nil {
			return object.Nil, nil
		}
		return object.ClassValue(cls(self).Super), nil
	})
	def(k, "class", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ClassValue(self.Class()), nil
	})
	def(k, "fields", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		names := []object.Value{}
		for _, f := range cls(self).Fields {
			names = append(names, object.Symbol(f.Name))
		}
		return object.NewList(names...), nil
	})
}

// FieldDecl is a field a class the Go binary defines declares: its name
// and its scalar type.
type FieldDecl struct{ Name, Type string }

// DeclareFields gives a class the Go binary defines its fields, numbered
// as a script's declaration numbers them, so that fields, with and == on
// its values work as on a script's class's. Its values do not cross to a
// host, to async, into a ref field or through Marshal: crossing asks for
// the digest a script's class has and a Go-defined one has not (#295).
func DeclareFields(c *object.Class, fields ...FieldDecl) {
	taken := func(number int64) bool {
		for _, f := range c.Fields {
			if f.Number == number {
				return true
			}
		}
		return false
	}
	for _, d := range fields {
		f := &object.Field{Name: d.Name, Number: protogen.FieldNumber(d.Name, taken), Type: object.ScalarField(d.Type), Owner: c}
		c.FieldIndex[d.Name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

// InheritFields gives a subclass the Go binary defines its superclass's
// fields.
func InheritFields(c *object.Class) {
	for _, f := range c.Super.Fields {
		c.FieldIndex[f.Name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

// Module is the module bound to a top-level constant, made and bound when
// there is none: what two libraries may share, as OpenSSL, which digest
// defines its HMAC and Digest under.
func Module(name string) *object.Class {
	if v, ok := object.ObjectClass.Consts[name]; ok {
		if c, isClass := v.AsClass(); isClass {
			return c
		}
	}
	m := object.NewModule(name)
	object.ObjectClass.Consts[name] = object.ClassValue(m)
	return m
}

// Forward is a method of a core class that a library written in cameo
// gives it, String#shellsplit for one: it calls the library module's
// function name with the receiver and the arguments, and, while the
// module is not defined, says to require the library.
func Forward(module, name, library string) object.NativeFn {
	return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m, ok := ctx.Const(module)
		if !ok {
			return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method '%s' for an instance of %s: require \"%s\" first", name, self.Class().QualifiedName(), library)
		}
		return ctx.CallWith(m, name, append([]object.Value{self}, args...), kw, block)
	}
}

// privateInRuby are the methods every class has that Ruby keeps private, so
// that instance_methods and methods leave them out as Ruby's do. cameo has
// no private methods: respond_to? still says true for them.
var privateInRuby = map[string]bool{
	"initialize": true, "initialize_copy": true, "method_missing": true, "respond_to_missing?": true,
}

// instanceMethodNames returns the names of the methods c's values answer:
// c's own, and with inherited those of its ancestors too, but those of a
// module of I/O the host does not allow (object.Class.IOKind).
func instanceMethodNames(ctx object.Context, c *object.Class, inherited bool) map[string]bool {
	names := map[string]bool{}
	classes := []*object.Class{c}
	if inherited {
		classes = c.Ancestors()
	}
	for _, k := range classes {
		if !ioAvailable(ctx, k) {
			continue
		}
		for n, m := range k.Methods {
			if !m.Private && !privateInRuby[n] {
				names[n] = true
			}
		}
	}
	return names
}

// singletonMethodNames returns the names of c's class methods: c's own,
// and with all those LookupSingleton finds too, its superclasses' and the
// methods of the modules they extend.
func singletonMethodNames(ctx object.Context, c *object.Class, all bool) map[string]bool {
	names := map[string]bool{}
	for k := c; k != nil; k = k.Super {
		for n := range k.SingletonMethods {
			names[n] = true
		}
		if !all {
			break
		}
		for _, e := range k.Extends {
			for n := range instanceMethodNames(ctx, e, true) {
				names[n] = true
			}
		}
	}
	return names
}

// ioAvailable reports whether the host allows the I/O of the module k,
// true for a class or module that does none.
func ioAvailable(ctx object.Context, k *object.Class) bool {
	return k.IOKind == "" || ctx.IOAllowed(k.IOKind)
}

// methodSymbols returns names as a List of Symbols, sorted.
func methodSymbols(names map[string]bool) object.Value {
	sorted := slices.Sorted(maps.Keys(names))
	out := make([]object.Value, len(sorted))
	for i, n := range sorted {
		out[i] = object.Symbol(n)
	}
	return object.NewList(out...)
}
