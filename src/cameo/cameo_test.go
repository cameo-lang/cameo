package cameo

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

const script = `
module Geo
  class Point
    int :x, :y
    def +(other) = Point.new(x: x + other.x, y: y + other.y)
    def to_s = "(#{x}, #{y})"
  end
end

def total(list) = list.sum
puts "loaded"
`

func load(t *testing.T) (*Program, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	prog, err := Load("t.cameo", script, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	return prog, &out
}

func TestCallWithGoValues(t *testing.T) {
	prog, out := load(t)
	if out.String() != "loaded\n" {
		t.Fatalf("output %q", out.String())
	}
	sum, err := prog.Call(nil, "total", []any{1, 2, 3.5})
	if err != nil {
		t.Fatal(err)
	}
	if sum != 6.5 {
		t.Fatalf("total: %#v", sum)
	}
	pt, err := prog.Call("Geo::Point", "new", map[string]any{"x": 1, "y": 2})
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := pt.(proto.Message)
	if !ok {
		t.Fatalf("an instance comes back as a proto.Message, got %T", pt)
	}
	if name := msg.ProtoReflect().Descriptor().FullName(); name != "geo.Point" {
		t.Fatalf("message %s", name)
	}
	twice, err := prog.Call(msg, "+", msg)
	if err != nil {
		t.Fatal(err)
	}
	s, err := prog.Call(twice, "to_s")
	if err != nil {
		t.Fatal(err)
	}
	if s != "(2, 4)" {
		t.Fatalf("to_s: %#v", s)
	}
	data, err := proto.Marshal(twice.(proto.Message))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty wire form")
	}
}

func TestProtoAndEnvelope(t *testing.T) {
	prog, _ := load(t)
	text, err := prog.Proto("Geo::Point")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "package geo;\n\nmessage Point {\n  optional int64 x = 1;\n") {
		t.Fatalf("proto text:\n%s", text)
	}
	files := prog.ProtoFiles()
	if _, ok := files["geo.proto"]; !ok {
		t.Fatalf("files: %v", files)
	}
	if !strings.Contains(files["cameo/value.proto"], "message Value {") {
		t.Fatal("the envelope is among the files")
	}
	data, err := prog.Encode(map[string]any{"k": []any{1, "x", nil}})
	if err != nil {
		t.Fatal(err)
	}
	back, err := prog.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := back.(map[any]any)
	if !ok || len(m) != 1 {
		t.Fatalf("decoded %#v", back)
	}
	list := m["k"].([]any)
	if list[0] != int64(1) || list[1] != "x" || list[2] != nil {
		t.Fatalf("decoded list %#v", list)
	}
}

func TestErrorsAreReported(t *testing.T) {
	prog, _ := load(t)
	_, err := prog.Call("Geo::Point", "nope")
	if err == nil || !strings.Contains(err.Error(), "undefined method 'nope'") {
		t.Fatalf("error: %v", err)
	}
	if _, err := Load("t.cameo", "a << 1\n", nil, nil); err == nil {
		t.Fatal("a rejected script does not load")
	}
	if _, err := prog.ToValue(struct{}{}); err == nil {
		t.Fatal("an unknown Go type does not convert")
	}
}

func TestRunContextStopsTheScript(t *testing.T) {
	// A script that never ends stops when its context does, in a loop, a
	// block or a method, past any rescue and in its workers.
	scripts := map[string]string{
		"while":  "while true\nend\n",
		"block":  "loop { }\n",
		"rescue": "begin\n  loop { }\nrescue Exception\n  puts :caught\nend\n",
		"method": "def spin(n) = n.times { }\nwhile true\n  spin(1)\nend\n",
		"worker": "class W\n  def go = loop { }\nend\nawait(async(W.new, :go))\n",
	}
	for name, src := range scripts {
		var out bytes.Buffer
		prog, err := Load(name+".cameo", src, &out, &out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		start := time.Now()
		err = prog.RunContext(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: RunContext returned %v, want the deadline", name, err)
		}
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("%s: stopped after %s", name, took)
		}
		if out.Len() > 0 {
			t.Errorf("%s: printed %q", name, out.String())
		}
	}
	// A script that ends returns as Run does.
	prog, err := Load("ok.cameo", "puts 1\n", &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.RunContext(context.Background()); err != nil {
		t.Fatalf("a script that ends: %v", err)
	}
}

func TestLastValue(t *testing.T) {
	// The last top-level expression, as p shows it; "" for nil.
	cases := map[string]string{
		"1 + 2\n":                               "3",
		"x = [1, 2]\nx + [3]\n":                 "[1, 2, 3]",
		"\"a\" * 2\n":                           `"aa"`,
		"{a: 1}\n":                              "{a: 1}",
		"puts 1\n":                              "",
		"def f = 1\n":                           ":f",
		"class P\n  int :x\nend\nP.new(x: 1)\n": "P(x: 1)",
	}
	for src, want := range cases {
		prog, err := Load("t.cameo", src, &bytes.Buffer{}, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if got, _ := prog.LastValue(); got != "" {
			t.Errorf("%q: %q before the run", src, got)
		}
		if err := prog.Run(); err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if got, err := prog.LastValue(); err != nil || got != want {
			t.Errorf("%q: LastValue is %q, %v; want %q", src, got, err, want)
		}
	}
}

func TestCallReturnsAHashWithSymbolKeysWithStringKeys(t *testing.T) {
	prog, err := Load("h.cameo", `def h = {a: 1, "b" => [2]}`, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := prog.Call(nil, "h")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[any]any)
	if !ok || len(m) != 2 || m["a"] != int64(1) {
		t.Fatalf("h: %#v", got)
	}
	if list, ok := m["b"].([]any); !ok || len(list) != 1 || list[0] != int64(2) {
		t.Fatalf("h[b]: %#v", m["b"])
	}
}

func TestCallRefusesAHashWhoseKeysHaveNoGoMapForm(t *testing.T) {
	prog, err := Load("h.cameo", `
def symbol_and_string = {a: 1, "a" => 2}
def range_key = {(1..2) => 1}
def binary_key = {"k".b => 1}
`, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for method, want := range map[string]string{
		"symbol_and_string": `both "a"`,
		"range_key":         "Range",
		"binary_key":        "[]uint8",
	} {
		_, err := prog.Call(nil, method)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want one naming %s", method, err, want)
		}
	}
}
