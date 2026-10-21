package aot_test

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/examples/aot/arith"
	"github.com/cameo-lang/cameo/examples/aot/blocks"
	"github.com/cameo-lang/cameo/examples/aot/classes"
	"github.com/cameo-lang/cameo/examples/aot/spin"
	"github.com/cameo-lang/cameo/src/aot"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/eval"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// examples are the generated packages, each built from <name>.rb by
//
//	cameo build examples/aot/<name>.rb examples/aot/<name> <name>
//
// The ones with a <name>.expected print it, compiled or not.
var examples = []struct {
	name, file, script string
}{
	{"arith", arith.File, arith.Script},
	{"blocks", blocks.File, blocks.Script},
	{"classes", classes.File, classes.Script},
	{"spin", spin.File, spin.Script},
}

// TestCompiledExamplesPrintWhatRubyPrints runs each example twice, with
// its compiled methods and on the interpreter alone, and holds both to the
// output Ruby gives, in <name>.expected.
func TestCompiledExamplesPrintWhatRubyPrints(t *testing.T) {
	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			want, err := os.ReadFile(ex.name + ".expected")
			if errors.Is(err, os.ErrNotExist) {
				t.Skip("the example does not end")
			}
			if err != nil {
				t.Fatal(err)
			}
			prog, err := syntax.Parse(ex.file, ex.script)
			if err != nil {
				t.Fatal(err)
			}
			if len(eval.CompiledFor(prog)) == 0 {
				t.Fatal("no compiled methods are registered for the script")
			}
			// A trailing nil makes another program, which nothing was
			// compiled for, and which prints the same.
			for _, run := range []struct{ how, src string }{{"compiled", ex.script}, {"interpreted", ex.script + "nil\n"}} {
				var out, errs bytes.Buffer
				if err := cameo.Run(ex.file, run.src, &out, &errs); err != nil {
					t.Fatalf("%s: %v\n%s", run.how, err, errs.String())
				}
				if out.String() != string(want) {
					t.Errorf("%s run printed\n%s\nwant\n%s", run.how, out.String(), want)
				}
			}
		})
	}
}

// TestCompiledExamplesAreUpToDate rebuilds each example and compares the
// Go with what is checked in.
func TestCompiledExamplesAreUpToDate(t *testing.T) {
	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			src, err := os.ReadFile(ex.name + ".rb")
			if err != nil {
				t.Fatal(err)
			}
			res, err := aot.Build(ex.name+".rb", string(src), aot.Options{Package: ex.name})
			if err != nil {
				t.Fatal(err)
			}
			have, err := os.ReadFile(filepath.Join(ex.name, ex.name+".go"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(res.Go, have) {
				t.Errorf("%s/%s.go is out of date; run: bin/cameo build examples/aot/%s.rb examples/aot/%s %s", ex.name, ex.name, ex.name, ex.name, ex.name)
			}
			if len(res.Compiled) == 0 {
				t.Errorf("nothing in %s.rb compiles", ex.name)
			}
		})
	}
}

// TestCompiledLoopsStop stops a run in a compiled loop that never ends.
func TestCompiledLoopsStop(t *testing.T) {
	var out bytes.Buffer
	prog, err := cameo.Load(spin.File, spin.Script, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- prog.RunContext(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("the run ended with %v, want the deadline", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the compiled loop did not stop")
	}
}

// TestCompiledMethodsGrowPast64Bits calls the compiled methods of arith
// directly, as the interpreter does, with Integers that outgrow 64 bits
// and with one beyond them, and holds that the Go answers rather than
// leaving the call to the interpreter.
func TestCompiledMethodsGrowPast64Bits(t *testing.T) {
	prog, err := syntax.Parse(arith.File, arith.Script)
	if err != nil {
		t.Fatal(err)
	}
	methods := eval.CompiledFor(prog)
	in := eval.New(&bytes.Buffer{}, &bytes.Buffer{})
	call := func(name string, args ...object.Value) (object.Value, error) {
		return methods[name](in, object.Nil, args, nil, object.Value{})
	}
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	v, err := call("power", object.Int(2), object.Int(64))
	if err != nil || !object.Equal(v, object.BigInt(two64)) {
		t.Errorf("power(2, 64) = %v, %v; want 2**64 from the Go", v, err)
	}
	v, err = call("floor_div", object.BigInt(two64), object.Int(1<<60))
	if err != nil || !object.Equal(v, object.Int(16)) {
		t.Errorf("floor_div(2**64, 2**60) = %v, %v; want 16 from the Go", v, err)
	}
	v, err = call("floor_mod", object.Int(-7), object.BigInt(two64))
	if err != nil || !object.Equal(v, object.BigInt(new(big.Int).Sub(two64, big.NewInt(7)))) {
		t.Errorf("floor_mod(-7, 2**64) = %v, %v; want 2**64 - 7 from the Go", v, err)
	}
	// A zero divisor and an argument that is no Integer stay with the
	// interpreter, which raises.
	for _, c := range []struct {
		name string
		args []object.Value
	}{{"floor_div", []object.Value{object.BigInt(two64), object.Int(0)}}, {"floor_div", []object.Value{object.Int(1), object.Int(0)}}, {"power", []object.Value{object.Float(2.5), object.Int(2)}}} {
		if _, err := call(c.name, c.args...); err != eval.ErrDeopt {
			t.Errorf("%s(%v) returned %v, want ErrDeopt", c.name, c.args, err)
		}
	}
}
