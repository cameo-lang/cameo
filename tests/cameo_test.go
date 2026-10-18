// Package tests holds cameo's Go tests: they import cameo's packages the way
// a program that embeds cameo does. The shell tests that drive the built
// binary are under integration/.
package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"

	"github.com/cameo-lang/cameo/src/cameo"
)

func TestVersionIsSet(t *testing.T) {
	assert.NotEquals(t, cameo.Version(), "", "cameo.Version() is empty")
}
