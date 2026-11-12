package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// A call that is a statement, whose value nothing reads, runs as any
// call.
func TestACallThatIsAStatementRuns(t *testing.T) {
	out := runScript(t, `
def twice(v) = v * 2
def noisy(v)
  puts "noisy #{v}"
  v
end
twice 2
noisy 3
noisy(4)
`)
	assert.Equals(t, out, `noisy 3
noisy 4
`)
}

// A call that drops the copy force_encoding returns is still refused.
func TestADroppedCopyIsStillRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "def f(s)\n  s.force_encoding(\"BINARY\")\n  1\nend\np f(\"abc\")\n", &stdout, &stderr)
	assert.True(t, err != nil && strings.Contains(err.Error(), "force_encoding returns a copy"))
}
