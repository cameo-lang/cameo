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
	mod.Def("echo", func(args []any, kwargs map[string]any) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, []any{args, kwargs})
		return map[string]any{"args": args, "kwargs": kwargs}, nil
	})
	mod.Def("fail", func(args []any, kwargs map[string]any) (any, error) {
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
