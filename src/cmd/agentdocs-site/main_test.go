package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/agentdocs"
	"github.com/cameo-lang/cameo/src/assert"
)

// The site gets each topic as the markdown cameo agent-docs prints, and
// an llms.txt that links every topic.
func TestTheSiteGetsTheTopicsAndAnLLMsTxt(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, write(dir))
	for _, topic := range agentdocs.Topics() {
		data, err := os.ReadFile(filepath.Join(dir, "agent-docs", topic.Name+".md"))
		assert.NoError(t, err)
		want, _ := agentdocs.Text(topic.Name)
		assert.Equals(t, string(data), want)
	}
	llms, err := os.ReadFile(filepath.Join(dir, "llms.txt"))
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(llms), "# cameo\n\n> "), "llms.txt starts with %q", string(llms)[:20])
	assert.Contains(t, string(llms), "- [async](agent-docs/async.md): async and await")
}
