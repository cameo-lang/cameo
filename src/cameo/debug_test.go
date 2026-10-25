package cameo

import (
	"bytes"
	"strings"
	"testing"
)

const debugScript = `def fib(n)
  return n if n < 2
  fib(n - 1) + fib(n - 2)
end

class Point
  int :x, :y
end

total = 0
[1, 2].each do |i|
  total += fib(i)
end
pt = Point.new(x: total, y: 5)
puts pt.x + pt.y
`

// debugRun runs the script under the debugger with the commands as its
// console, and returns the console's output, the script's output and
// Run's error.
func debugRun(t *testing.T, src, commands string) (console, output string, err error) {
	t.Helper()
	var out, con bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Debug(strings.NewReader(commands), &con)
	err = prog.Run()
	return con.String(), out.String(), err
}

// TestDebuggerStopsStepsAndContinues proves the debugger stops before the
// first statement, next steps over a method definition, a class body and a
// call, step enters a call, and continue runs the rest of the script.
func TestDebuggerStopsStepsAndContinues(t *testing.T) {
	console, output, err := debugRun(t, debugScript, "n\nn\nn\nn\ns\ns\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"=>#0\t<main> at t.cameo:1",
		"=>#0\t<main> at t.cameo:6",  // next over def fib
		"=>#0\t<main> at t.cameo:10", // next over the class body
		"=>#0\t<main> at t.cameo:11",
		"=>#0\t<main> at t.cameo:14", // next over each and its block
		"=>#0\t<main> at t.cameo:15",
	} {
		if !strings.Contains(console, want) {
			t.Errorf("console lacks %q:\n%s", want, console)
		}
	}
	if output != "7\n" {
		t.Fatalf("script output %q", output)
	}
}

// TestDebuggerBreaksInAMethodAndShowsItsFrame proves a breakpoint on a
// method stops at its first line, the backtrace names the frames with
// their arguments, info lists the locals, p evaluates in the frame, and up
// moves to the caller.
func TestDebuggerBreaksInAMethodAndShowsItsFrame(t *testing.T) {
	console, output, err := debugRun(t, debugScript, "b fib\nc\nbt\ninfo\np n * 100\nup\np total\np i\nfinish\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#1  BP - Method  fib",
		"=>#0\tfib(n=1) at t.cameo:2",
		"  #1\tblock in <main> at t.cameo:12",
		"  #2\t<main> at t.cameo:11",
		"n = 1\n",
		"=> 100\n",
		"=> 0\n", // total, in the block frame
		"=> 1\n", // i
	} {
		if !strings.Contains(console, want) {
			t.Errorf("console lacks %q:\n%s", want, console)
		}
	}
	// finish from fib(1) returned to the block; continue then stopped at
	// fib(2), the breakpoint, and ran on from there until the end.
	if !strings.Contains(console, "=>#0\tfib(n=2) at t.cameo:2") || output != "7\n" {
		t.Fatalf("after finish:\n%s\noutput %q", console, output)
	}
}

// TestDebuggerBreaksOnALineAndListsAround proves a line breakpoint stops
// each time the line runs, list shows the source around it, and delete
// removes it.
func TestDebuggerBreaksOnALineAndListsAround(t *testing.T) {
	console, _, err := debugRun(t, debugScript, "b 12\nc\nl\ninfo\nc\ninfo\ndel 1\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(console, "=>#0\tblock in <main> at t.cameo:12") != 2 {
		t.Fatalf("the line stopped %d times:\n%s", strings.Count(console, "at t.cameo:12"), console)
	}
	for _, want := range []string{"[8, 15] in t.cameo", "=>   12|   total += fib(i)", "i = 1\ntotal = 0\n", "i = 2\ntotal = 1\n", "deleted breakpoint #1"} {
		if !strings.Contains(console, want) {
			t.Errorf("console lacks %q:\n%s", want, console)
		}
	}
}

// TestDebuggerQuitEndsTheRun proves quit stops the script, and Quit tells
// the host so.
func TestDebuggerQuitEndsTheRun(t *testing.T) {
	var out, con bytes.Buffer
	prog, err := Load("t.cameo", "puts 1\nputs 2\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Debug(strings.NewReader("n\nq\n"), &con)
	err = prog.Run()
	if err == nil || !prog.Quit() {
		t.Fatalf("Run returned %v, Quit %v", err, prog.Quit())
	}
	if out.String() != "1\n" {
		t.Fatalf("output %q", out.String())
	}
}

// TestDebuggerFromRequire proves a script that requires "debug" stops at
// binding.break and debugger on the console the host gave it, runs through
// them without a console, and that without the library they are undefined.
func TestDebuggerFromRequire(t *testing.T) {
	src := "require \"debug\"\na = 1\nbinding.break\nputs a\ndebugger\nputs a + 1\n"
	var out, con bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Console(strings.NewReader("p a\nc\np a * 10\nc\n"), &con)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"=>#0\t<main> at t.cameo:4", "=> 1\n", "=>#0\t<main> at t.cameo:6", "=> 10\n"} {
		if !strings.Contains(con.String(), want) {
			t.Errorf("console lacks %q:\n%s", want, con.String())
		}
	}
	if out.String() != "1\n2\n" {
		t.Fatalf("output %q", out.String())
	}

	out.Reset()
	prog, err = Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil || out.String() != "1\n2\n" {
		t.Fatalf("without a console: %v, output %q", err, out.String())
	}

	out.Reset()
	prog, err = Load("t.cameo", "debugger\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	err = prog.Run()
	if err == nil || !strings.Contains(err.Error(), "undefined method 'debugger'") {
		t.Fatalf("without require: %v", err)
	}
}

// TestDebuggerDefinesDebuggerWithoutRequire proves a script run under
// cameo debug stops at debugger and binding.break without requiring
// "debug", as under rdbg.
func TestDebuggerDefinesDebuggerWithoutRequire(t *testing.T) {
	console, output, err := debugRun(t, "a = 1\ndebugger\nputs a\nbinding.break\nputs a + 1\n", "c\np a\nc\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if output != "1\n2\n" || strings.Count(console, "=>#0\t<main> at t.cameo:") != 3 || !strings.Contains(console, "=> 1\n") {
		t.Fatalf("output %q, console:\n%s", output, console)
	}
}

// TestDebuggerBreaksOnANamespacedMethod proves break takes Foo::Bar.baz
// and Foo::Bar#qux, and file:line still names a line.
func TestDebuggerBreaksOnANamespacedMethod(t *testing.T) {
	src := "module Foo\n  class Bar\n    def self.baz = 3\n    def qux = 4\n  end\nend\nputs Foo::Bar.baz\nputs Foo::Bar.new.qux\n"
	console, output, err := debugRun(t, src, "b Foo::Bar.baz\nb Foo::Bar#qux\nb t.cameo:8\nc\nc\nc\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if output != "3\n4\n" {
		t.Fatalf("output %q, console:\n%s", output, console)
	}
	for _, want := range []string{"#1  BP - Method  Foo::Bar.baz", "#2  BP - Method  Foo::Bar#qux", "#3  BP - Line  t.cameo:8", "=>#0\tFoo::Bar.baz() at t.cameo:3", "=>#0\tFoo::Bar#qux() at t.cameo:4", "=>#0\t<main> at t.cameo:8"} {
		if !strings.Contains(console, want) {
			t.Errorf("console lacks %q:\n%s", want, console)
		}
	}
}

// TestDebuggerNamesABlockInALambda proves a stop inside a lambda names the
// frame the lambda was written in, as a block's.
func TestDebuggerNamesABlockInALambda(t *testing.T) {
	console, output, err := debugRun(t, "def g(n)\n  l = -> { debugger; n }\n  l.call\nend\nputs g(5)\n", "c\nbt\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if output != "5\n" || !strings.Contains(console, "=>#0\tblock in g at t.cameo:2") || !strings.Contains(console, "#1\tg(n=5) at t.cameo:3") {
		t.Fatalf("output %q, console:\n%s", output, console)
	}
}
