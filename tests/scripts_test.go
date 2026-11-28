package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// endMarker separates a script case from the output it prints. cameo does
// not parse __END__, so the runner cuts the file there.
const endMarker = "\n__END__\n"

// TestScripts runs each scripts/*__test.cameo.rb and holds its output to
// the text after its __END__ line.
func TestScripts(t *testing.T) {
	scripts, err := filepath.Glob("scripts/*__test.cameo.rb")
	assert.NoError(t, err)
	assert.True(t, len(scripts) > 0, "no scripts in tests/scripts")
	for _, script := range scripts {
		name := strings.TrimSuffix(filepath.Base(script), "__test.cameo.rb")
		t.Run(name, func(t *testing.T) {
			text, err := os.ReadFile(script)
			assert.NoError(t, err)
			src, want, found := strings.Cut(string(text), endMarker)
			assert.True(t, found, "%s has no __END__ line", script)
			var stdout, stderr bytes.Buffer
			if err := cameo.RunFromString(filepath.Base(script), src, cameoio.Streams(&stdout, &stderr)); err != nil {
				t.Fatalf("%s failed: %v\nstderr: %s", script, err, stderr.String())
			}
			assert.Equals(t, stdout.String(), want)
		})
	}
}
