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
// returns what the script printed. The interpreter keeps the activation
// that makes a tail call, so that each call the script makes is counted.
func probed(t *testing.T, src string) string {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	var out bytes.Buffer
	in := New(&out, &out)
	in.KeepTailFrames(TailFramesKept)
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
"a".gsub("a") { |x| down(1) }
`)
	assert.Equals(t, got, "1 activations, 1 loops\n"+
		// The statement, then down(3) to down(0).
		"5 activations, 1 loops\n"+
		// The statement, twice, each_one, its block, the block of twice.
		"5 activations, 1 loops\n"+
		// The statement; then, in the loop gsub runs its block in, the
		// block, down(1) and down(0).
		"4 activations, 2 loops\n")
}

// A builtin that is a stepper is an activation between its block's
// calls: its block runs in the one loop, where the block of a builtin
// that is none runs in a loop of its own. Called from Go, as an
// Enumerator calls it, a stepper runs to its end.
func TestAStepperAndItsBlockTakeTurnsInOneLoop(t *testing.T) {
	got := probed(t, `2.times { |i| Probe.state if i == 1 }
1.upto(1) { 3.downto(3) { loop { Probe.state; break } } }
"a".gsub("a") { 1.times { Probe.state } }
[1].each { [2].map { |x| [x].select { Probe.state } } }
p 3.times.map { |i| i * 2 }, 2.upto(4).to_a
p 5.times { |i| break i * 10 if i == 2 }
p (loop { raise StopIteration })
p (1.times { raise "out" } rescue "rescued")
`)
	assert.Equals(t, got,
		// The statement, times, its block.
		"3 activations, 1 loops\n"+
			// The statement, then three builtins, each with its block.
			"7 activations, 1 loops\n"+
			// gsub calls its block from Go; times is a stepper inside it.
			"4 activations, 2 loops\n"+
			// The walks of a List are steppers too.
			"7 activations, 1 loops\n"+
			"[0, 2, 4]\n[2, 3, 4]\n20\nnil\n\"rescued\"\n")
}

// A script recurses as deep as its memory, not as Go's stack, goes: far
// past where a call of the evaluator per call ended the process, up to
// the depth where a call raises.
func TestRecursionIsBoundedByACountNotByTheGoStack(t *testing.T) {
	prints(t, `def count(n) = n == 0 ? 0 : 1 + count(n - 1)
p count(150_000)
def forever(n) = 1 + forever(n + 1)
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

// A tail call does not keep the call that makes it: a method that calls
// itself, or two that call each other, a million calls deep run in a
// stack that does not grow. The call may pass keywords, be made in a
// method with more variables than a scope holds, in a method with a
// block in it, by a lambda that calls itself, or be new. A block that
// returns from its method, a call under an ensure and a call whose
// value is used are as they were.
func TestTailCallsKeepNoFrame(t *testing.T) {
	prints(t, `def count(n, acc)
  return acc if n == 0
  count(n - 1, acc + 1)
end
def even?(n) = n == 0 ? true : odd?(n - 1)
def odd?(n) = n == 0 ? false : even?(n - 1)
class Walker
  int :limit
  def walk(n, seen)
    if n >= limit
      seen
    else
      step(n, seen)
    end
  end
  def step(n, seen)
    extra = [n].map { |v| v % 7 == 0 ? 1 : 0 }.sum
    walk(n + 1, seen + extra)
  end
end
def with_keywords(n, total: 0)
  return total if n == 0
  with_keywords(n - 1, total: total + n)
end
def wide(n, a, b, c, d, e)
  f = a + b
  return f + c + d + e if n == 0
  wide(n - 1, b, c, d, e, f % 10)
end
def find_first(list)
  list.each { |x| return x * 2 if x > 2 }
end
def builds(n) = Walker.new(limit: n)
def through_lambda(n)
  step = ->(k, acc) { k == 0 ? acc : step.call(k - 1, acc + 2) }
  step.call(n, 0)
end
def guarded(n)
  begin
    n == 0 ? :done : guarded(n - 1)
  ensure
    nil
  end
end
def not_tail(n) = n == 0 ? 0 : 1 + not_tail(n - 1)
p count(1_000_000, 0), even?(300_001), odd?(300_001)
p Walker.new(limit: 200_000).walk(0, 0), with_keywords(100_000), wide(300_000, 1, 2, 3, 4, 5)
p find_first([1, 2, 3, 4]), builds(3).walk(0, 0), through_lambda(300_000)
p guarded(1000), not_tail(1000)
p [1, 2, 3].map { |x| count(x, 10) }
`, "1000000\nfalse\ntrue\n28572\n5000050000\n24\n6\n1\n600000\n:done\n1000\n[11, 12, 13]\n")
}

// A method that calls itself in tail position is a loop in one
// activation, whatever the depth; a tail call of another kind takes the
// place of its caller. With KeepTailFrames the calls stay, a row of them
// no longer than the number kept.
func TestTailCallsAndTheActivationsTheyKeep(t *testing.T) {
	src := `def down(n) = n == 0 ? Probe.state : down(n - 1)
def hop(n) = n == 0 ? Probe.state : skip(n - 1, k: 1)
def skip(n, k:) = hop(n)
down(1000)
hop(1000)
[1].each { |x| down(x) }
`
	run := func(keep int) string {
		prog, err := syntax.Parse("t.cameo", src)
		assert.NoError(t, err)
		var out bytes.Buffer
		in := New(&out, &out)
		in.KeepTailFrames(keep)
		probe := object.NewModule("Probe")
		probe.DefineSingleton("state", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			fmt.Fprintf(&out, "%d activations\n", len(ctx.(*Interp).acts))
			return object.Nil, nil
		})
		in.SetConst("Probe", object.ClassValue(probe))
		assert.NoError(t, in.Run("t.cameo", prog))
		return out.String()
	}
	// The statement and the one call that runs; inside each, the
	// stepper, its block's place and the call.
	assert.Equals(t, run(0), "2 activations\n2 activations\n3 activations\n")
	// The statement, the row that is kept, and the call that runs. A
	// row is cut back to the number kept when it is a quarter longer,
	// so it is that number or up to a quarter more.
	kept := func(out string) (first, second, third int) {
		_, err := fmt.Sscanf(out, "%d activations\n%d activations\n%d activations\n", &first, &second, &third)
		assert.NoError(t, err)
		return first - 2, second - 2, third
	}
	for _, keep := range []int{4, TailFramesKept} {
		first, second, third := kept(run(keep))
		assert.True(t, first >= keep && first <= keep+keep/4, fmt.Sprint(first))
		assert.True(t, second >= keep && second <= keep+keep/4, fmt.Sprint(second))
		assert.Equals(t, third, 5)
	}
}

// A block written in a method that made a tail call has nowhere to
// return to once it is called: the method ended with its tail call.
func TestAProcReturnsToNoMethodThatMadeATailCall(t *testing.T) {
	prints(t, `def later(pr) = pr.call
def early
  pr = proc { return 1 }
  later(pr)
end
begin
  early
rescue LocalJumpError => e
  p e.message
end
`, "\"unexpected return\"\n")
}
