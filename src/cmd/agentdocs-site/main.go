// Command agentdocs-site writes cameo's documentation for agents
// (src/agentdocs) into the site, as the Pages workflow publishes it:
//
//	agentdocs-site doc/gh_pages
//
// writes each topic as agent-docs/<topic>.md, the markdown cameo
// agent-docs prints, and llms.txt, the overview's lead and a link to each
// topic, where tools that read a site for a model look first.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cameo-lang/cameo/src/agentdocs"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: agentdocs-site <site directory>")
		os.Exit(2)
	}
	if err := write(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "agentdocs-site:", err)
		os.Exit(1)
	}
}

// write writes the topics and llms.txt into the site at dir.
func write(dir string) error {
	topics := filepath.Join(dir, "agent-docs")
	if err := os.MkdirAll(topics, 0o755); err != nil {
		return err
	}
	for _, t := range agentdocs.Topics() {
		text, err := agentdocs.Text(t.Name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(topics, t.Name+".md"), []byte(text), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "llms.txt"), []byte(llms()), 0o644)
}

// llms is the site's llms.txt: a title, what cameo is, and a link to each
// topic with what it says.
func llms() string {
	var b strings.Builder
	b.WriteString("# cameo\n\n")
	b.WriteString("> cameo is a small language with Ruby's syntax, run by an interpreter written in Go. ")
	b.WriteString("Write Ruby, and leave out everything that changes a value in place: every value is immutable.\n\n")
	b.WriteString("The same guide is built into cameo: `cameo agent-docs [topic]` prints it, and a Go program ")
	b.WriteString("reaches it with `agentdocs.Text` (github.com/cameo-lang/cameo/src/agentdocs).\n\n")
	b.WriteString("## Guide for agents\n\n")
	for _, t := range agentdocs.Topics() {
		fmt.Fprintf(&b, "- [%s](agent-docs/%s.md): %s\n", t.Name, t.Name, t.Summary)
	}
	b.WriteString("\n## More\n\n")
	b.WriteString("- [Ruby vs cameo](ruby.html): each difference beside its Ruby\n")
	b.WriteString("- [Embedding in Go](go.html): the Go package in full\n")
	return b.String()
}
