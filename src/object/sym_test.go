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
