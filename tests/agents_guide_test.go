package tests

import (
	"bytes"
	"regexp"
	"strconv"
	"testing"

	"github.com/cameo-lang/cameo/src/agentdocs"
	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// guideSnippet is a Ruby block of a topic of agentdocs.
var guideSnippet = regexp.MustCompile("(?s)```ruby\n(.*?)```")

// TestTheAgentDocsSnippetsRun runs every Ruby block of agentdocs, the
// documentation for models that write cameo (cameo agent-docs): a model
// writes what the topics show, so each block must be a script cameo
// accepts and runs to its end.
func TestTheAgentDocsSnippetsRun(t *testing.T) {
	count := 0
	for _, topic := range agentdocs.Topics() {
		text, err := agentdocs.Text(topic.Name)
		assert.NoError(t, err)
		for i, m := range guideSnippet.FindAllStringSubmatch(text, -1) {
			count++
			t.Run(topic.Name+"/"+strconv.Itoa(i+1), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				assert.NoError(t, cameo.Run(topic.Name+".md", m[1], &stdout, &stderr))
				assert.Equals(t, stderr.String(), "")
			})
		}
	}
	assert.True(t, count >= 6, "found %d snippets in agentdocs", count)
}
