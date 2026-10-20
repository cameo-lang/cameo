package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// asyncExamples are the scripts of examples/async, which run unchanged on
// cameo and, with ruby/lib/cameo/runner.rb, on CRuby.
func asyncExamples(t *testing.T) []string {
	t.Helper()
	scripts, err := filepath.Glob("../examples/async/*.rb")
	assert.NoError(t, err)
	assert.True(t, len(scripts) > 0, "no scripts in examples/async")
	return scripts
}

func expectedOf(t *testing.T, script string) string {
	t.Helper()
	want, err := os.ReadFile(strings.TrimSuffix(script, ".rb") + ".expected")
	assert.NoError(t, err)
	return string(want)
}

// TestAsyncExamplesOnCameo runs each async example on cameo and holds it to
// its expected output.
func TestAsyncExamplesOnCameo(t *testing.T) {
	for _, script := range asyncExamples(t) {
		t.Run(filepath.Base(script), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.NoError(t, cameo.RunFile(script, &stdout, &stderr))
			assert.Equals(t, stderr.String(), "")
			assert.Equals(t, stdout.String(), expectedOf(t, script))
		})
	}
}

// TestAsyncExamplesOnRuby runs each async example on CRuby with
// Cameo::Runner from ruby/lib and holds it to the output cameo prints. It
// is skipped where no ruby is on PATH.
func TestAsyncExamplesOnRuby(t *testing.T) {
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("no ruby on PATH")
	}
	for _, script := range asyncExamples(t) {
		t.Run(filepath.Base(script), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(ruby, "-I", "../ruby/lib", script)
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			assert.NoError(t, cmd.Run(), "%s", stderr.String())
			assert.Equals(t, stderr.String(), "")
			assert.Equals(t, stdout.String(), expectedOf(t, script))
		})
	}
}
