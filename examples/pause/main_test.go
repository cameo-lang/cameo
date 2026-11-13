package main

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A run paused on step 3 goes on with step 4 in another program, made of
// the bytes of its state alone.
func TestARunGoesOnFromItsState(t *testing.T) {
	var first, second bytes.Buffer
	data, err := firstHalf(3, &first, &first)
	assert.NoError(t, err)
	assert.Equals(t, first.String(), "step 1: total 1\nstep 2: total 5\nstep 3: total 14\npaused before line 9\n")
	assert.NoError(t, secondHalf(data, &second, &second))
	assert.Equals(t, second.String(), "step 4: total 30\nstep 5: total 55\ndone: 55\n")
	// The same state starts the second half again.
	second.Reset()
	assert.NoError(t, secondHalf(data, &second, &second))
	assert.Equals(t, second.String(), "step 4: total 30\nstep 5: total 55\ndone: 55\n")
}
