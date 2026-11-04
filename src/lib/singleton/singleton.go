// Package singleton is Ruby's singleton library, built into cameo:
// include Singleton in a class, and Klass.instance is its one instance,
// made by initialize the first time and the same value after, while new
// and allocate are refused, and dup and clone on the instance.
//
// It follows singleton 0.2.0, the version Ruby 3.4 ships. The instance is
// a value, as every instance is, so there is one and it never changes;
// a subclass has an instance of its own, as in Ruby.
package singleton

import (
	"sync"

	"github.com/cameo-lang/cameo/src/object"
)

var module = object.NewModule("Singleton")

var (
	mu        sync.Mutex
	instances = map[*object.Class]object.Value{}
)

// Install defines Singleton. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("Singleton", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.2.0")
	m.Included = included
	for _, name := range []string{"dup", "clone"} {
		name := name
		m.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Value{}, ctx.Raise(object.TypeError, "can't %s instance of singleton %s", name, self.Class().QualifiedName())
		})
	}
}

// included gives the including class instance, and takes new and allocate
// from it.
func included(ctx object.Context, into *object.Class) error {
	into.DefineSingleton("instance", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cls := self.Cls()
		mu.Lock()
		v, ok := instances[cls]
		mu.Unlock()
		if ok {
			return v, nil
		}
		// Class#new as the core defines it, since the class's own is gone.
		v, err := object.ClassClass.Methods["new"].Native(ctx, self, nil, nil, object.Value{})
		if err != nil {
			return object.Value{}, err
		}
		mu.Lock()
		if prior, ok := instances[cls]; ok {
			v = prior
		} else {
			instances[cls] = v
		}
		mu.Unlock()
		return v, nil
	})
	for _, name := range []string{"new", "allocate"} {
		name := name
		into.DefineSingleton(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Value{}, ctx.Raise(object.NoMethodError, "private method '%s' called for class %s", name, self.Cls().QualifiedName())
		})
	}
	return nil
}
