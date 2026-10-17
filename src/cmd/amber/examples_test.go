package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ruby-parser/parser"
)

// examplesDir is the corpus of scripts that specify the language, relative
// to this package.
const examplesDir = "../../../examples"

// TestExamplesParse holds every example script to the parser: a script the
// front end cannot read cannot specify anything. Each example also needs its
// expected output beside it, or the corpus stops being a specification.
func TestExamplesParse(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.arb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no example scripts under %s", examplesDir)
	}
	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			src, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parser.Parse(string(src)); err != nil {
				t.Fatalf("does not parse: %v", err)
			}
			expected := strings.TrimSuffix(script, ".arb") + ".expected"
			if _, err := os.Stat(expected); err != nil {
				t.Fatalf("missing expected output %s", filepath.Base(expected))
			}
		})
	}
}

// TestCheckRejectsSyntaxError proves the check command fails on a script the
// parser rejects, so a green run on the corpus means something.
func TestCheckRejectsSyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.arb")
	if err := os.WriteFile(path, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := check(path); err == nil {
		t.Fatal("check accepted a script with a syntax error")
	}
	if status := run([]string{"check", path}); status != 1 {
		t.Fatalf("exit status %d for a syntax error, want 1", status)
	}
	if status := run([]string{path}); status != 2 {
		t.Fatalf("exit status %d for running a script, want 2 while unimplemented", status)
	}
}
