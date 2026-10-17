// Package tests holds amber's Go tests: they import amber's packages the way
// a program that embeds amber does. The shell tests that drive the built
// binary are under integration/.
package tests

import (
	"testing"

	"github.com/radiospiel/simple-go/src/assert"

	"github.com/radiospiel/amber/src/amber"
)

func TestVersionIsSet(t *testing.T) {
	assert.NotEquals(t, amber.Version(), "", "amber.Version() is empty")
}
