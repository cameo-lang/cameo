package tests

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// guideSnippet is a Ruby block of agents.md.
var guideSnippet = regexp.MustCompile("(?s)```ruby\n(.*?)```")

// TestTheAgentsGuideSnippetsRun runs every Ruby block of agents.md, the
// guide for models that write cameo: a model writes what the guide shows,
// so each block must be a script cameo accepts and runs to its end.
func TestTheAgentsGuideSnippetsRun(t *testing.T) {
	guide, err := os.ReadFile("../agents.md")
	assert.NoError(t, err)
	snippets := guideSnippet.FindAllStringSubmatch(string(guide), -1)
	assert.True(t, len(snippets) >= 3, "found %d snippets in agents.md", len(snippets))
	for i, m := range snippets {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.NoError(t, cameo.Run("agents.md", m[1], &stdout, &stderr))
			assert.Equals(t, stderr.String(), "")
		})
	}
}
