package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIrbEvaluatesLinesAndWaitsForWholeExpressions proves cameo irb prints
// each line's value, keeps variables between lines, reads on while an
// expression is open, reports an exception and goes on, and ends at exit.
func TestIrbEvaluatesLinesAndWaitsForWholeExpressions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := strings.NewReader("x = 41\ndef inc(n)\n  n + 1\nend\ninc(x)\n[x,\n 2].sum\nraise \"boom\"\nexit\nputs :unreached\n")
	if status := irb("", in, &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	want := "cameo(main):001> => 41\n" +
		"cameo(main):002> cameo(main):002* cameo(main):002* => :inc\n" +
		"cameo(main):003> => 42\n" +
		"cameo(main):004> cameo(main):004* => 43\n" +
		"cameo(main):005> (irb):1: boom (RuntimeError)\n" +
		"cameo(main):006> "
	if stdout.String() != want {
		t.Fatalf("output:\n%q\nwant:\n%q", stdout.String(), want)
	}
}

// TestIrbRunsTheScriptFirst proves cameo irb with a script runs it, and the
// lines typed in see its definitions.
func TestIrbRunsTheScriptFirst(t *testing.T) {
	script := filepath.Join(t.TempDir(), "s.cameo")
	if err := os.WriteFile(script, []byte("def twice(n) = n * 2\nbase = 10\nputs :loaded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := irb(script, strings.NewReader("twice(base)\n"), &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "loaded\ncameo(main):001> => 20\n") {
		t.Fatalf("output %q", stdout.String())
	}
}

// TestIrbEndsWithTheScriptsExit proves a script that exits ends the
// session with its status, before any line is read.
func TestIrbEndsWithTheScriptsExit(t *testing.T) {
	script := filepath.Join(t.TempDir(), "s.cameo")
	if err := os.WriteFile(script, []byte("puts :before\nexit 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := irb(script, strings.NewReader("puts :unreached\n"), &stdout, &stderr); status != 3 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if stdout.String() != "before\n" || stderr.Len() != 0 {
		t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

// TestDebugCommandStopsFirstAndQuits proves cameo debug stops before the
// first statement, and quitting the debugger exits 0 without an error.
func TestDebugCommandStopsFirstAndQuits(t *testing.T) {
	script := filepath.Join(t.TempDir(), "s.cameo")
	if err := os.WriteFile(script, []byte("puts 1\nputs 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := debugScript(script, false, strings.NewReader("n\nq\n"), &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "=>#0\t<main> at "+script+":1") || !strings.Contains(stdout.String(), "(rdbg) 1\n") || strings.Contains(stdout.String(), "(rdbg) 2\n") {
		t.Fatalf("output %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr %q", stderr.String())
	}
}
