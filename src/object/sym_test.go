package object

import (
	"sync"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The operators and the other well-known names have the number of their
// constant, and read back as their name.
func TestWellKnownSymsAreConstants(t *testing.T) {
	assert.Equals(t, Intern("+"), SymAdd)
	assert.Equals(t, Intern("!="), SymNe)
	assert.Equals(t, Intern("initialize"), SymInitialize)
	assert.Equals(t, SymCmp.String(), "<=>")
	assert.Equals(t, SymNone.String(), "")
	for i, name := range wellKnown {
		assert.Equals(t, Intern(name), Sym(i))
	}
}

// A name is numbered once, also when goroutines intern it at the same
// time, and a name nothing interned has no Sym.
func TestInternNumbersANameOnce(t *testing.T) {
	_, ok := LookupSym("sym_test_never_interned")
	assert.False(t, ok)

	var wg sync.WaitGroup
	got := make([]Sym, 16)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = Intern("sym_test_interned_once")
		}()
	}
	wg.Wait()
	for _, s := range got {
		assert.Equals(t, s, got[0])
	}
	assert.True(t, got[0] >= symWellKnown)
	assert.Equals(t, got[0].String(), "sym_test_interned_once")
	s, ok := LookupSym("sym_test_interned_once")
	assert.True(t, ok)
	assert.Equals(t, s, got[0])
}

// A lookup by Sym finds what a lookup by name finds, a miss too, and
// forgets it when a method is defined: on the class, on its superclass,
// or as a class method.
func TestLookupBySymFollowsDefinitions(t *testing.T) {
	noop := func(Context, Value, []Value, *Map, Value) (Value, error) { return Nil, nil }
	base := NewClass("SymLookupBase", ObjectClass)
	sub := NewClass("SymLookupSub", base)
	name := Intern("sym_lookup_method")

	assert.True(t, sub.LookupSym(name) == nil)
	assert.True(t, sub.LookupSingletonSym(name) == nil)

	base.Define("sym_lookup_method", 0, noop)
	inherited := sub.LookupSym(name)
	assert.True(t, inherited != nil && inherited == sub.Lookup("sym_lookup_method"))
	assert.True(t, inherited.Owner == base)

	sub.Define("sym_lookup_method", 0, noop)
	assert.True(t, sub.LookupSym(name).Owner == sub)

	base.DefineSingleton("sym_lookup_method", 0, noop)
	found := sub.LookupSingletonSym(name)
	assert.True(t, found != nil && found == sub.LookupSingleton("sym_lookup_method"))
	// The instance method is still found after the class method was.
	assert.True(t, sub.LookupSym(name).Owner == sub)
}
