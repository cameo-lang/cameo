package stdlib

import (
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
	def(m, "instance_methods", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		names := []object.Value{}
		for _, n := range cls(self).MethodNames() {
			names = append(names, object.Symbol(n))
		}
		return object.NewList(names...), nil
	})
	def(m, "method_defined?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(cls(self).Lookup(name) != nil), nil
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
