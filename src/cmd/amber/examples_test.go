package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// examplesDir is the corpus of scripts that specify the language, relative
// to this package.
const examplesDir = "../../../examples"

// TestExamplesCheck holds every example script to the front end: a script
// the checker rejects cannot specify anything. Each example also needs its
// expected output beside it, or the corpus stops being a specification.
func TestExamplesCheck(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.arb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no example scripts under %s", examplesDir)
	}
	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			if err := check(script); err != nil {
				t.Fatalf("rejected: %v", err)
			}
			expected := strings.TrimSuffix(script, ".arb") + ".expected"
			if _, err := os.Stat(expected); err != nil {
				t.Fatalf("missing expected output %s", filepath.Base(expected))
			}
		})
	}
}

// TestCheckReportsErrors proves the check command fails on a script the
// parser rejects and on one the checker rejects, so a green run on the
// corpus means something.
func TestCheckReportsErrors(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.arb")
	if err := os.WriteFile(broken, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := check(broken); err == nil {
		t.Fatal("check accepted a script with a syntax error")
	}
	mutating := filepath.Join(dir, "mutating.arb")
	if err := os.WriteFile(mutating, []byte("a = []\na << 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := check(mutating)
	if err == nil || !strings.Contains(err.Error(), "mutating.arb:2:") {
		t.Fatalf("check on a mutating script: %v", err)
	}
	if status := run([]string{"check", broken}); status != 1 {
		t.Fatalf("exit status %d for a syntax error, want 1", status)
	}
	if status := run([]string{broken}); status != 2 {
		t.Fatalf("exit status %d for running a script, want 2 while unimplemented", status)
	}
}
