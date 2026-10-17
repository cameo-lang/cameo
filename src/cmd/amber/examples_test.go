package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/radiospiel/amber/src/amber"
)

// examplesDir is the corpus of scripts that specify the language, relative
// to this package.
const examplesDir = "../../../examples"

// pending names the examples that need a milestone not built yet, with the
// milestone. A pending example must still fail: when it passes, it comes
// off this list.
var pending = map[string]string{
	"point.arb":             "milestone 4: classes with fields",
	"nested_assignment.arb": "milestone 4: classes with fields",
	"redefinition.arb":      "milestone 4: classes with fields",
	"inheritance.arb":       "milestone 4: classes with fields",
	"modules.arb":           "milestone 4: modules",
	"proto.arb":             "milestone 5: protobuf",
	"async.arb":             "milestone 6: async",
}

// TestExamples runs every example script and holds its standard output to
// the expected file beside it. Pending examples are run too, and must fail.
func TestExamples(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.arb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no example scripts under %s", examplesDir)
	}
	for _, script := range scripts {
		name := filepath.Base(script)
		t.Run(name, func(t *testing.T) {
			expected, err := os.ReadFile(strings.TrimSuffix(script, ".arb") + ".expected")
			if err != nil {
				t.Fatalf("missing expected output: %v", err)
			}
			var stdout, stderr bytes.Buffer
			runErr := amber.RunFile(script, &stdout, &stderr)
			if why, isPending := pending[name]; isPending {
				if runErr == nil && stdout.String() == string(expected) {
					t.Fatalf("passes, but is listed as pending (%s): take it off the list", why)
				}
				t.Skipf("pending: %s", why)
			}
			if runErr != nil {
				t.Fatalf("failed: %v\nstdout so far:\n%s", runErr, stdout.String())
			}
			if stdout.String() != string(expected) {
				t.Fatalf("output differs\n--- got ---\n%s--- want ---\n%s", stdout.String(), expected)
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
	if status := run([]string{broken}); status != 1 {
		t.Fatalf("exit status %d for running a broken script, want 1", status)
	}
}

// TestRunReportsExceptions proves an uncaught exception names its line and
// class and fails the run.
func TestRunReportsExceptions(t *testing.T) {
	script := filepath.Join(t.TempDir(), "boom.arb")
	if err := os.WriteFile(script, []byte("puts 1\nraise ArgumentError, \"boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := amber.RunFile(script, &stdout, &stderr)
	if err == nil {
		t.Fatal("an uncaught exception did not fail the run")
	}
	if !strings.Contains(err.Error(), "boom.arb:2: boom (ArgumentError)") {
		t.Fatalf("error message: %v", err)
	}
	if stdout.String() != "1\n" {
		t.Fatalf("output before the exception: %q", stdout.String())
	}
}
