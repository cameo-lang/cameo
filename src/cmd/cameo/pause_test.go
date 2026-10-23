package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/cameo"
)

// pausedDump runs a script that pauses itself through a host method and
// returns the path of its dump.
func pausedDump(t *testing.T) string {
	t.Helper()
	src := "n = 1\nputs n\nTest.pause\nputs n + 1\n"
	prog, err := cameo.Load("count.cameo", src, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	prog.Module("Test").Def("pause", func([]any, map[string]any) (any, error) {
		prog.Pause()
		return nil, nil
	})
	if err := prog.Run(); !errors.Is(err, cameo.ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "count.dump")
	if err := os.WriteFile(path, dump.Data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestResumeCommandContinuesTheScript proves `cameo resume` runs the rest
// of a paused script and exits 0.
func TestResumeCommandContinuesTheScript(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := resume(pausedDump(t), false, &stdout, &stderr); status != 0 {
		t.Fatalf("exit status %d, stderr:\n%s", status, stderr.String())
	}
	if stdout.String() != "2\n" {
		t.Fatalf("output %q, want %q", stdout.String(), "2\n")
	}
}

// TestInspectCommandShowsTheDump proves `cameo inspect` prints where the
// script paused and what it held.
func TestInspectCommandShowsTheDump(t *testing.T) {
	var out bytes.Buffer
	if err := inspect(pausedDump(t), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"script: count.cameo\n", "paused: before line 4\n", "  n = 1\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("inspect printed:\n%s\nwithout %q", out.String(), want)
		}
	}
}

// TestResumeCommandRejectsAMissingDump proves `cameo resume` on a path
// that does not exist fails with status 1 and says so.
func TestResumeCommandRejectsAMissingDump(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := resume(filepath.Join(t.TempDir(), "none.dump"), false, &stdout, &stderr); status != 1 {
		t.Fatalf("exit status %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "none.dump") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}
