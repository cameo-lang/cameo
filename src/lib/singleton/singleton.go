// Package singleton is Ruby's singleton library, built into cameo:
// include Singleton in a class, and Klass.instance is its one instance,
// made by initialize the first time and the same value after, while new
// and allocate are refused, and dup and clone on the instance.
//
// It follows singleton 0.3.0, the version Ruby 3.4 ships. The instance is
// a value, as every instance is, so there is one and it never changes;
// a subclass has an instance of its own, as in Ruby. The instance is
// kept on the class, which a reopening shares, under one lock, so
// initialize runs once however the first calls race.
package singleton

import (
	"sync"

	"github.com/cameo-lang/cameo/src/object"
)

var module = object.NewModule("Singleton")

// state is what Singleton keeps on a class: its instance, once made, under
// the lock that makes it once.
type state struct {
	mu       sync.Mutex
	made     bool
	instance object.Value
}

// Install defines Singleton. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Singleton", object.ClassValue(m))
	m.Consts["VERSION"] = object.NewString("0.3.0", object.ASCII)
	m.Included = included
	m.Extended = func(ctx object.Context, into *object.Class) error {
		return ctx.Raise(object.NoMethodError, "undefined method 'extend_object' for module Singleton")
	}
	for _, name := range []string{"dup", "clone"} {
		name := name
		m.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Value{}, ctx.Raise(object.TypeError, "can't %s instance of singleton %s", name, self.Class().QualifiedName())
		})
	}
}

// stateOf is the class's Singleton state, made on the first call.
func stateOf(cls *object.Class) *state {
	if cls.Library == nil {
		cls.Library = map[string]any{}
	}
	st, ok := cls.Library["singleton"].(*state)
	if !ok {
		st = &state{}
		cls.Library["singleton"] = st
	}
	return st
}

// className is the class as a message names it: its name, or Ruby's form
// for one without.
func className(ctx object.Context, self object.Value) string {
	if name := self.Cls().QualifiedName(); name != "" {
		return name
	}
	s, _ := ctx.Inspect(self)
	return s
}

// included gives the including class instance, and takes new and allocate
// from it; a module cannot include Singleton, as in Ruby.
func included(ctx object.Context, into *object.Class) error {
	if into.IsModule {
		return ctx.Raise(object.TypeError, "Inclusion of the OO-Singleton module in module %s", into.QualifiedName())
	}
	// The state is a subclass's own: a subclass has an instance of its own.
	into.Library = map[string]any{"singleton": &state{}}
	into.DefineSingleton("instance", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		st := stateOf(cls)
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.made {
			// A reopened class is a copy; the instance is the class's as it
			// now is, with the fields it had.
			if inst := st.instance.Instance(); inst.Cls != cls {
				moved := *inst
				moved.Cls = cls
				st.instance = object.InstanceValue(&moved)
			}
			return st.instance, nil
		}
		// Class#new as the core defines it, since the class's own is gone.
		v, err := object.ClassClass.Methods["new"].Native(ctx, self, nil, nil, object.Value{})
		if err != nil {
			return object.Value{}, err
		}
		st.instance, st.made = v, true
		return v, nil
	})
	for _, name := range []string{"new", "allocate"} {
		name := name
		into.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Value{}, ctx.Raise(object.NoMethodError, "private method '%s' called for class %s", name, className(ctx, self))
		})
	}
	return nil
}
