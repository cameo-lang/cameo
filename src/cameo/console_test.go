package cameo

import (
	"bytes"
	"errors"
	"testing"
)

// TestEvalKeepsTheTopLevel proves Eval runs a snippet at the script's top
// level: a variable bound by one snippet is a variable to the next, a
// method defined by one is callable from the next, and an exception comes
// back as a RunError naming the script.
func TestEvalKeepsTheTopLevel(t *testing.T) {
	prog, err := Load("(irb)", "", &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ src, want string }{
		{"x = 41\n", "41"},
		{"def inc(n)\n  n + 1\nend\n", ":inc"},
		{"inc(x)\n", "42"},
		{"y = [x, 2].sum\ny\n", "43"},
		{"nil\n", "nil"},
		{"class P\n  int :a\nend\nP.new(a: x).a\n", "41"},
	} {
		got, err := prog.Eval(step.src)
		if err != nil || got != step.want {
			t.Fatalf("%q: %q, %v; want %q", step.src, got, err, step.want)
		}
	}
	_, err = prog.Eval("raise ArgumentError, \"boom\"\n")
	var runErr *RunError
	if !errors.As(err, &runErr) || runErr.Message != "(irb):1: boom (ArgumentError)" {
		t.Fatalf("raise: %v", err)
	}
}
