package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/cameo"
)

// examplesDir is the corpus of scripts that specify the language, relative
// to this package.
const examplesDir = "../../../examples"

// pending names the examples that need a milestone not built yet, with the
// milestone. A pending example must still fail: when it passes, it comes
// off this list.
var pending = map[string]string{}

// TestExamples runs every example script and holds its standard output to
// the expected file beside it. Pending examples are run too, and must fail.
func TestExamples(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.cameo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no example scripts under %s", examplesDir)
	}
	for _, script := range scripts {
		name := filepath.Base(script)
		t.Run(name, func(t *testing.T) {
			expected, err := os.ReadFile(strings.TrimSuffix(script, ".cameo") + ".expected")
			if err != nil {
				t.Fatalf("missing expected output: %v", err)
			}
			var stdout, stderr bytes.Buffer
			runErr := cameo.RunFile(script, &stdout, &stderr)
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
	broken := filepath.Join(dir, "broken.cameo")
	if err := os.WriteFile(broken, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := check(broken); err == nil {
		t.Fatal("check accepted a script with a syntax error")
	}
	mutating := filepath.Join(dir, "mutating.cameo")
	if err := os.WriteFile(mutating, []byte("A = []\nA << 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := check(mutating)
	if err == nil || !strings.Contains(err.Error(), "mutating.cameo:2:") {
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
	script := filepath.Join(t.TempDir(), "boom.cameo")
	if err := os.WriteFile(script, []byte("puts 1\nraise ArgumentError, \"boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := cameo.RunFile(script, &stdout, &stderr)
	if err == nil {
		t.Fatal("an uncaught exception did not fail the run")
	}
	if !strings.Contains(err.Error(), "boom.cameo:2: boom (ArgumentError)") {
		t.Fatalf("error message: %v", err)
	}
	if stdout.String() != "1\n" {
		t.Fatalf("output before the exception: %q", stdout.String())
	}
}

// TestProtoCommand proves `cameo proto` prints one file per package, with
// the envelope, and writes them into a directory when given one.
func TestProtoCommand(t *testing.T) {
	var out bytes.Buffer
	if err := protoFiles(filepath.Join(examplesDir, "proto.cameo"), "", &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"// cameo.proto\n", "// cameo/value.proto\n", "// geo.proto\n", "message Person {", "message Address {", "message Point {", "message Value {"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, out.String())
		}
	}
	dir := t.TempDir()
	out.Reset()
	if err := protoFiles(filepath.Join(examplesDir, "proto.cameo"), dir, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "geo.proto")); err != nil {
		t.Fatalf("geo.proto not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cameo", "value.proto")); err != nil {
		t.Fatalf("cameo/value.proto not written: %v", err)
	}
}
