package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cameo-lang/cameo/src/aot"
	"github.com/cameo-lang/cameo/src/assert"
)

// TestPerformanceScriptsCompile builds every script under performance/ and
// holds that cameo build compiles at least one of its methods. A script
// whose methods all stayed on the interpreter would time the interpreter
// twice, and its table would show nothing about compiled code.
func TestPerformanceScriptsCompile(t *testing.T) {
	scripts, err := filepath.Glob("performance/*/*.rb")
	assert.NoError(t, err)
	assert.True(t, len(scripts) > 0, "no scripts under performance/")
	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			src, err := os.ReadFile(script)
			assert.NoError(t, err)
			res, err := aot.Build(script, string(src), aot.Options{})
			assert.NoError(t, err)
			assert.True(t, len(res.Compiled) > 0, "nothing compiles: %v", res.Left)
		})
	}
}
