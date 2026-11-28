package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// A call that drops the copy force_encoding returns is still refused.
func TestADroppedCopyIsStillRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", "def f(s)\n  s.force_encoding(\"BINARY\")\n  1\nend\np f(\"abc\")\n", cameoio.Streams(&stdout, &stderr))
	assert.True(t, err != nil && strings.Contains(err.Error(), "force_encoding returns a copy"))
}
