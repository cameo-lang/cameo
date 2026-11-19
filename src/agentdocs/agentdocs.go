// Package agentdocs is cameo's documentation for models that write cameo
// or embed it: short markdown topics, built into the binary, which a tool
// prints for an agent that asks (cameo agent-docs [topic]). The topics
// are in topics/; Text("") is the overview with the list of topics.
//
//	text, err := agentdocs.Text("async")   // one topic
//	for _, t := range agentdocs.Topics() { fmt.Println(t.Name, t.Summary) }
package agentdocs

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed topics/*.md
var files embed.FS

// Topic is a topic of the documentation: Name is what Text takes, and
// Summary what the topic is about, in a line.
type Topic struct {
	Name, Summary string
}

// topics are the topics in the order the overview lists them, after the
// overview itself.
var topics = []Topic{
	{"overview", "what cameo is, the one rule, and the commands to check and run a script"},
	{"rules", "what to leave out of Ruby, and what to write instead"},
	{"classes", "classes with typed fields, and with for a changed copy"},
	{"collections", "computing with collections without accumulators"},
	{"async", "async and await, for work that does not depend on other work"},
	{"text", "text and bytes: Strings that are UTF-8 or binary"},
	{"recursion", "tail calls: a recursion that ends in the call runs at any depth"},
	{"io", "what I/O a script may do, and who allows it"},
	{"stdlib", "the libraries a script may require"},
	{"embedding", "running cameo from Go: Load, Run, Call and host modules"},
	{"pause", "pausing a running script into bytes and resuming it"},
	{"checklist", "what to check before handing a script over"},
}

// Topics lists the topics, the overview first.
func Topics() []Topic { return append([]Topic(nil), topics...) }

// Text returns the markdown of the topic name, and for "" the overview
// followed by the list of topics. A name it does not know is an error
// that lists the topics.
func Text(name string) (string, error) {
	if name == "" {
		return overview(), nil
	}
	for _, t := range topics {
		if t.Name == name {
			return read(name), nil
		}
	}
	names := make([]string, len(topics))
	for i, t := range topics {
		names[i] = t.Name
	}
	return "", fmt.Errorf("no topic %q: the topics are %s", name, strings.Join(names, ", "))
}

// overview is the overview topic and the list of the others.
func overview() string {
	var b strings.Builder
	b.WriteString(read("overview"))
	b.WriteString("\n## Topics\n\n")
	for _, t := range topics[1:] {
		fmt.Fprintf(&b, "- `%s`: %s\n", t.Name, t.Summary)
	}
	b.WriteString("\nAsk for one with `cameo agent-docs <topic>`.\n")
	return b.String()
}

// read returns the file of the topic name; every topic has one, which a
// test checks.
func read(name string) string {
	data, err := files.ReadFile("topics/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("agentdocs: the topic %s has no file: %v", name, err))
	}
	return string(data)
}
