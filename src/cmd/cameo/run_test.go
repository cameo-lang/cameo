package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunCommandRunsTheCompiledProgram proves `cameo run` builds the
// program `cameo build` writes and runs it: the output is the script's,
// with the compiled fib past 64 bits on its *big.Int twin.
func TestRunCommandRunsTheCompiledProgram(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fib.rb")
	src := "def fib(n)\n  n < 2 ? n : fib(n - 1) + fib(n - 2)\nend\n\ndef fact(n)\n  n < 2 ? 1 : n * fact(n - 1)\nend\n\nputs fib(20)\nputs fact(25)\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := runCompiled(script, &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if got, want := stdout.String(), "6765\n15511210043330985984000000\n"; got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}

// TestRunCommandFailsAsTheScriptDoes proves an uncaught exception in the
// compiled program fails `cameo run` with status 1 and the interpreter's
// message.
func TestRunCommandFailsAsTheScriptDoes(t *testing.T) {
	script := filepath.Join(t.TempDir(), "boom.rb")
	if err := os.WriteFile(script, []byte("puts 1\nraise ArgumentError, \"boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := runCompiled(script, &stdout, &stderr); status != 1 {
		t.Fatalf("exit status %d, want 1; stderr:\n%s", status, stderr.String())
	}
	if stdout.String() != "1\n" {
		t.Fatalf("output before the exception: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "boom.rb:2: boom (ArgumentError)") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

// TestRunCommandRejectsABrokenScript proves a script the compiler cannot
// parse fails before any Go build, with the parser's error.
func TestRunCommandRejectsABrokenScript(t *testing.T) {
	script := filepath.Join(t.TempDir(), "broken.rb")
	if err := os.WriteFile(script, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := runCompiled(script, &stdout, &stderr); status != 1 {
		t.Fatalf("exit status %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "broken.rb:") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}
