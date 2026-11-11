package cameo

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A variable bound to nil, false, an empty string or an empty list is a
// variable still after a resume: each is a value whose envelope wire form
// is empty or has no bytes of its own.
func TestResumeKeepsVariablesBoundToEmptyValues(t *testing.T) {
	prog, _ := pausing(t, `a = nil
b = false
c = ""
d = []
N = nil
Test.pause
p a, b, c, d, N
`)
	err := prog.Run()
	assert.True(t, errors.Is(err, ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	var resumed bytes.Buffer
	cont, err := Resume(dump.Data, &resumed, &resumed)
	assert.NoError(t, err)
	assert.NoError(t, cont.Run())
	assert.Equals(t, resumed.String(), "nil\nfalse\n\"\"\n[]\nnil\n")
}
