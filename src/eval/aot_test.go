package eval

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// runCompiled registers methods for src's program and runs it.
func runCompiled(t *testing.T, src string, methods Compiled) string {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	if err != nil {
		t.Fatal(err)
	}
	RegisterCompiled(syntax.Digest(prog), methods)
	var out bytes.Buffer
	in := New(&out, &out)
	if err := in.Run("t.cameo", prog); err != nil {
		t.Fatal(in.FormatError(err))
	}
	return out.String()
}

func TestCompiledMethodsRunForTheirProgramOnly(t *testing.T) {
	answer := func(object.Context, object.Value, []object.Value, *object.Map, object.Value) (object.Value, error) {
		return object.Int(42), nil
	}
	got := runCompiled(t, "def f(n)\n  n\nend\nputs f(1)\nputs send(:f, 2)\n", Compiled{"f": answer})
	if got != "42\n42\n" {
		t.Errorf("the compiled program printed %q, want the compiled answer twice", got)
	}
	// The same method in another program is the interpreter's.
	expectOutput(t, "def f(n)\n  n\nend\nputs f(1)\nputs f(2)\n", "1\n2\n")
}

func TestCompiledMethodsLeaveCallsToTheInterpreter(t *testing.T) {
	calls := 0
	leave := func(object.Context, object.Value, []object.Value, *object.Map, object.Value) (object.Value, error) {
		calls++
		return object.Value{}, ErrDeopt
	}
	src := "def down(n)\n  n == 0 ? 0 : 1 + down(n - 1)\nend\nputs down(5)\nputs down(3)\n"
	if got := runCompiled(t, src, Compiled{"down": leave}); got != "5\n3\n" {
		t.Errorf("printed %q, want the interpreter's answers", got)
	}
	// Each call from the top level tries the Go once; the recursion
	// below a call that left runs on the interpreter.
	if calls != 2 {
		t.Errorf("the compiled method ran %d times, want 2", calls)
	}
}

func TestCompiledClassMethodsRunForTheirClass(t *testing.T) {
	answer := func(object.Context, object.Value, []object.Value, *object.Map, object.Value) (object.Value, error) {
		return object.Int(42), nil
	}
	src := "class A\n  def f(n)\n    n\n  end\nend\nclass B\n  def f(n)\n    n\n  end\nend\nputs A.new.f(1)\nputs B.new.f(2)\n"
	if got := runCompiled(t, src, Compiled{"A#f": answer}); got != "42\n2\n" {
		t.Errorf("printed %q, want the compiled answer for A and the interpreter's for B", got)
	}
}
