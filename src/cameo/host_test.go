package cameo

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// host loads src with a module Spicy whose echo returns what it was
// given, and whose fail raises Spicy::ToolError.
func host(t *testing.T, src string) (*Program, *bytes.Buffer, *[][]any) {
	t.Helper()
	var out bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var calls [][]any
	mod := prog.Module("Spicy")
	toolError := mod.Exception("ToolError")
	mod.Def("echo", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, []any{args, kwargs})
		return map[string]any{"args": args, "kwargs": kwargs}, nil
	})
	mod.Def("fail", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		if len(args) > 0 {
			return nil, toolError.New("no %v", args[0])
		}
		return nil, errors.New("plain")
	})
	return prog, &out, &calls
}

func TestHostMethodGetsPlainValues(t *testing.T) {
	prog, out, calls := host(t, `r = Spicy.echo("a", :b, [1, :c], {x: 1, "y" => :z}, {1 => 2}, q: :sym)
p r["args"][1], r["kwargs"]["q"]`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	want := []any{
		[]any{"a", "b", []any{int64(1), "c"}, map[string]any{"x": int64(1), "y": "z"}, map[any]any{int64(1): int64(2)}},
		map[string]any{"q": "sym"},
	}
	if !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("call %#v", (*calls)[0])
	}
	if out.String() != "\"b\"\n\"sym\"\n" {
		t.Errorf("output %q", out.String())
	}
}

func TestHostMethodRaises(t *testing.T) {
	prog, out, _ := host(t, `begin
  Spicy.fail("x")
rescue Spicy::ToolError => e
  puts "#{e.class}: #{e.message}"
end
begin
  Spicy.fail
rescue => e
  puts "#{e.class}: #{e.message}"
end
Spicy.fail(2)`)
	err := prog.Run()
	if err == nil || err.Error() != "t.cameo:11: no 2 (Spicy::ToolError)" {
		t.Errorf("err %v", err)
	}
	if out.String() != "Spicy::ToolError: no x\nRuntimeError: plain\n" {
		t.Errorf("output %q", out.String())
	}
}

func TestHostModuleCannotBeRedefined(t *testing.T) {
	for _, src := range []string{"module Spicy\nend", "class Spicy\nend"} {
		prog, _, _ := host(t, src)
		err := prog.Run()
		if err == nil || !strings.Contains(err.Error(), "cannot redefine the module Spicy, which the host defines") {
			t.Errorf("%q: %v", src, err)
		}
	}
	prog, _, _ := host(t, "")
	if prog.Module("Spicy").cls != prog.Module("Spicy").cls {
		t.Error("Module made a second Spicy")
	}
}

func TestHostMethodFromWorker(t *testing.T) {
	prog, out, calls := host(t, `include Cameo
class Job
  int :factor
  def run(n) = Spicy.echo(n)["args"].first * factor
end
job = Job.new(factor: 2)
ps = (1..3).map { |n| async job, :run, n }
p ps.map { |pr| await pr }`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[2, 4, 6]\n" || len(*calls) != 3 {
		t.Errorf("output %q, %d calls", out.String(), len(*calls))
	}
}

// TestHostMethodGetsItsBlock proves a host method gets the block of the
// call as a *Block it can call with Go values, and nil without one.
func TestHostMethodGetsItsBlock(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", `
Spicy.twice(21) { |x| puts x * 2 }
p Spicy.twice(1)
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Module("Spicy").Def("twice", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		if block == nil {
			return "no block", nil
		}
		for i := 0; i < 2; i++ {
			if _, err := block.Call(args[0]); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "42\n42\n\"no block\"\n" {
		t.Errorf("output %q", out.String())
	}
}

// TestProcArgumentIsABlockAndGoesBackAsAProc proves a Proc among the
// arguments arrives as a *Block whose Call converts its result, and that
// a *Block a host method returns is the Proc again.
func TestProcArgumentIsABlockAndGoesBackAsAProc(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", `
f = Spicy.keep(proc { |x| x + 1 })
p f.call(41)
p Spicy.run(2)
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	var kept *Block
	spicy := prog.Module("Spicy")
	spicy.Def("keep", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		b, ok := args[0].(*Block)
		if !ok {
			return nil, errors.New("not a block")
		}
		kept = b
		return b, nil
	})
	spicy.Def("run", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		return kept.Call(args[0])
	})
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "42\n3\n" {
		t.Errorf("output %q", out.String())
	}
	v, err := kept.Call(int64(9))
	if err != nil || v != int64(10) {
		t.Errorf("Call from the host after the run: %v, %v", v, err)
	}
}

// TestBlockCallReportsTheException proves an exception a block raises
// comes back from Call as a RunError with the script's line.
func TestBlockCallReportsTheException(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", `
Spicy.later { |x| raise ArgumentError, "no #{x}" }
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	var later *Block
	prog.Module("Spicy").Def("later", func(args []any, kwargs map[string]any, block *Block) (any, error) {
		later = block
		return nil, nil
	})
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	_, err = later.Call("luck")
	var runErr *RunError
	if !errors.As(err, &runErr) || err.Error() != "t.cameo:2: no luck (ArgumentError)" {
		t.Errorf("Call: %v", err)
	}
}
