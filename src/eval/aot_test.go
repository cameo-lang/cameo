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

func TestConstructBuildsAnInstanceAsNewDoes(t *testing.T) {
	prog, err := syntax.Parse("t.cameo", "class P\n  float :x\n  int :n, default: 7\n  int32 :small\nend\n")
	if err != nil {
		t.Fatal(err)
	}
	in := New(&bytes.Buffer{}, &bytes.Buffer{})
	if err := in.Run("t.cameo", prog); err != nil {
		t.Fatal(in.FormatError(err))
	}
	cls := ClassNamed(in, "P")
	if cls == nil || ClassNamed(in, "Q") != nil {
		t.Fatalf("ClassNamed found %v for P and %v for Q", cls, ClassNamed(in, "Q"))
	}
	inst, err := Construct(in, cls, []object.Value{object.Int(2)})
	if err != nil {
		t.Fatal(err)
	}
	if !object.Equal(inst.Fields[0], object.Float(2)) || !object.Equal(inst.Fields[1], object.Int(7)) || !inst.Fields[2].IsNil() {
		t.Errorf("the fields are %v, %v, %v; want 2.0, the default 7 and nil", inst.Fields[0], inst.Fields[1], inst.Fields[2])
	}
	if _, err := Construct(in, cls, []object.Value{{}, {}, object.Int(1 << 40)}); err != ErrDeopt {
		t.Errorf("a value past int32 gave %v, want ErrDeopt", err)
	}
	if _, err := Construct(in, nil, nil); err != ErrDeopt {
		t.Errorf("no class gave %v, want ErrDeopt", err)
	}
}
