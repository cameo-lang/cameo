package eval

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/syntax"
)

// onBoth runs src on the IR evaluator and on the tree walker, holds both
// to the output want, and returns how many of the script's methods, blocks
// and top-level statements ran on the IR and how many on the tree walker.
func onBoth(t *testing.T, src, want string) (compiled, walked int) {
	t.Helper()
	runOn := func(treeWalker bool) (string, *Interp) {
		TreeWalkerOnly.Store(treeWalker)
		defer TreeWalkerOnly.Store(false)
		prog, err := syntax.Parse("t.cameo", src)
		assert.NoError(t, err)
		var out bytes.Buffer
		in := New(&out, &out)
		if err := in.Run("t.cameo", prog); err != nil {
			out.WriteString(in.FormatError(err) + "\n")
		}
		return out.String(), in
	}
	walkedOut, _ := runOn(true)
	assert.Equals(t, walkedOut, want, "on the tree walker")
	irOut, in := runOn(false)
	assert.Equals(t, irOut, want, "on the IR")
	return in.IRCounts()
}

func TestStatementsRunOnTheIR(t *testing.T) {
	compiled, walked := onBoth(t, `a = 3
b = a * 2 + 1
puts b
puts "a is #{a}, b is #{b}"
puts [a, *[b, 9], :sym].inspect
puts({x: a, **{y: b}}.inspect)
puts (1..a).to_a.inspect, (1...a).to_a.inspect
puts a > 2 && b > 2, a > 5 || nil.inspect, !a
`, "7\na is 3, b is 7\n[3, 7, 9, :sym]\n{x: 3, y: 7}\n[1, 2, 3]\n[1, 2]\ntrue\nnil\nfalse\n")
	assert.Equals(t, compiled, 8)
	assert.Equals(t, walked, 0)
}

func TestAMethodRunsOnTheIRWhenItsDefinitionDoesNotCompileYet(t *testing.T) {
	compiled, walked := onBoth(t, `def fib(n)
  if n < 2
    n
  else
    fib(n - 1) + fib(n - 2)
  end
end
puts fib(15)
`, "610\n")
	// The def statement stays on the tree walker; the method's body and
	// the puts statement run on the IR.
	assert.Equals(t, compiled, 2)
	assert.Equals(t, walked, 1)
}

func TestLoopsAndJumpsRunOnTheIR(t *testing.T) {
	_, walked := onBoth(t, `i = 0
total = 0
while i < 10
  i += 1
  next if i == 3
  break if i == 8
  total += i
end
puts total
puts (while false do end).inspect
`, "25\nnil\n")
	assert.Equals(t, walked, 0)
}

func TestBlocksRunOnTheIR(t *testing.T) {
	_, walked := onBoth(t, `puts [1, 2, 3].map { |x| x * 2 }.inspect
double = ->(x) { x * 2 }
puts double.(4), double.lambda?, double.arity
add = proc { |a, b = 10, *rest| a + b + rest.sum }
puts add.call(1), add.call(1, 2, 3), add.arity
fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }
puts fact.(5)
puts [3, 1, 2].sort_by { |x| -x }.first
puts [[1, 2], [3, 4]].map { |a, b| a + b }.inspect
puts [1, 2, 3].each_with_index.map { |x, i| x * i }.inspect
puts (1..3).reduce(0) { |sum, x| sum + x }
puts [1, 2, 3, 4].find { |x| x > 2 }
`, "[2, 4, 6]\n8\ntrue\n1\n11\n6\n-2\n120\n3\n[3, 7]\n[0, 2, 6]\n6\n3\n")
	assert.Equals(t, walked, 0)
}

func TestCallsRunOnTheIR(t *testing.T) {
	onBoth(t, `def greet(name, greeting = "hello", *rest, punct: "!", **opts, &blk)
  s = "#{greeting} #{name}#{punct}"
  s = blk.call(s) if blk
  return s, rest, opts
end
def each_twice
  return :none unless block_given?
  yield 1
  yield 2, 3
end
def pass(*args, **kw) = greet(*args, **kw)
puts greet("ann").inspect
puts greet("bob", "hi", 1, 2, punct: "?", x: 1) { |s| s.upcase }.inspect
puts pass("cy", punct: ".").inspect
up = :upcase
puts ["a", "b"].map(&up).inspect
puts each_twice.inspect
each_twice { |a, b| puts [a, b].inspect }
x = nil
puts x&.length.inspect, "abc"&.length
puts 7 / 2, 7 % 3, 2**10, 1.5 + 2, 10.fdiv(4)
puts [1, 2].frozen?
`, `["hello ann!", [], {}]
["HI BOB?", [1, 2], {x: 1}]
["hello cy.", [], {}]
["A", "B"]
:none
[1, nil]
[2, 3]
nil
3
3
1
1024
3.5
2.5
true
`)
}

func TestErrorsOnTheIRNameTheirLine(t *testing.T) {
	onBoth(t, `def f(x)
  x + nope
end
f(1)
`, "t.cameo:2: undefined local variable or method 'nope' for main (NameError)\n")
	onBoth(t, "x = 1\ny = x.missing(2)\n", "t.cameo:2: undefined method 'missing' for an instance of Integer (NoMethodError)\n")
	onBoth(t, "def f = yield\nf\n", "t.cameo:1: no block given (yield) (LocalJumpError)\n")
	onBoth(t, "a = [1]\na << 2\n", "t.cameo:2: cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (CameoError)\n")
}
