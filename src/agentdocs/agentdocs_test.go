package agentdocs

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Every topic has a file, and every file is a topic.
func TestEveryTopicHasAFile(t *testing.T) {
	entries, err := fs.ReadDir(files, "topics")
	assert.NoError(t, err)
	assert.Equals(t, len(entries), len(topics))
	for _, t2 := range topics {
		text, err := Text(t2.Name)
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(text, "# "), "the topic %s starts with no title", t2.Name)
	}
}

// Text("") is the overview and the list of the other topics.
func TestTheOverviewListsTheTopics(t *testing.T) {
	text, err := Text("")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(text, "# cameo for agents"), "no overview: %q", text[:40])
	for _, t2 := range topics[1:] {
		assert.Contains(t, text, "- `"+t2.Name+"`: "+t2.Summary)
	}
}

// A topic it does not know is an error that names the topics.
func TestAnUnknownTopicNamesTheTopics(t *testing.T) {
	_, err := Text("threads")
	assert.Contains(t, err.Error(), `no topic "threads": the topics are overview, rules, classes`)
}
