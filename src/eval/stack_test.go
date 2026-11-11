package eval

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// probed runs src with a module Probe whose method state prints how many
// activations the interpreter is running and in how many loops, and
// returns what the script printed.
func probed(t *testing.T, src string) string {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	var out bytes.Buffer
	in := New(&out, &out)
	probe := object.NewModule("Probe")
	probe.DefineSingleton("state", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := ctx.(*Interp)
		fmt.Fprintf(&out, "%d activations, %d loops\n", len(in.acts), in.loops)
		return object.Nil, nil
	})
	in.SetConst("Probe", object.ClassValue(probe))
	if err := in.Run("t.cameo", prog); err != nil {
		out.WriteString(in.FormatError(err) + "\n")
	}
	return out.String()
}

// A call of a method a script defined, and a yield to a block a script
// wrote, are activations of the one loop: nothing of them is on the Go
// stack. A builtin that calls a block does so from Go, in a loop of its
// own.
func TestCallsAreActivationsOfOneLoop(t *testing.T) {
	got := probed(t, `def down(n) = n == 0 ? Probe.state : down(n - 1)
def each_one = yield(1)
def twice = each_one { |x| yield(x) }
Probe.state
down(3)
twice { |x| Probe.state }
[1].each { |x| down(1) }
`)
	assert.Equals(t, got, "1 activations, 1 loops\n"+
		// The statement, then down(3) to down(0).
		"5 activations, 1 loops\n"+
		// The statement, twice, each_one, its block, the block of twice.
		"5 activations, 1 loops\n"+
		// The statement; then, in the loop each runs its block in, the
		// block, down(1) and down(0).
		"4 activations, 2 loops\n")
}

// A script recurses as deep as its memory, not as Go's stack, goes: far
// past where a call of the evaluator per call ended the process, up to
// the depth where a call raises.
func TestRecursionIsBoundedByACountNotByTheGoStack(t *testing.T) {
	prints(t, `def count(n) = n == 0 ? 0 : 1 + count(n - 1)
p count(150_000)
def forever(n) = forever(n + 1)
begin
  forever(0)
rescue SystemStackError => e
  p e.message
end
p count(10)
`, "150000\n\"stack level too deep\"\n10\n")
}

// What ends a call is the same as when a call was a call of the
// evaluator: a return out of a block, a break out of a block given to a
// method, next and redo, a lambda's return, an ensure on the way.
func TestCompletionsCrossActivations(t *testing.T) {
	prints(t, `def each3
  yield 1
  yield 2
  yield 3
ensure
  puts "each3 done"
end
def first_even
  each3 { |x| return x if x.even? }
  nil
end
p first_even
p each3 { |x| break x * 10 if x == 2 }
p each3 { |x| next x }
def pass = yield
def outer
  pass { pass { return :deep } }
  :not_reached
end
p outer
l = ->(x) { return x + 1 }
p l.call(1), l.(2)
def maker = proc { return 1 }
begin
  maker.call
rescue LocalJumpError => e
  p e.message
end
require "stringio"
again = StringIO.new("nynn")
r = each3 do |x|
  puts "pass #{x}"
  redo if again.getc == "y"
  x
end
p r
def raiser = raise(ArgumentError, "from raiser")
def caller1 = raiser
begin
  caller1
rescue => e
  p [e.message, e.class]
end
`, "each3 done\n2\neach3 done\n20\neach3 done\n3\n:deep\n2\n3\n\"unexpected return\"\npass 1\npass 2\npass 2\npass 3\neach3 done\n3\n[\"from raiser\", ArgumentError]\n")
}
