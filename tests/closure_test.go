package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// A block captures the values of its variables when it is made, not the
// variables: these outputs are the specification. A rebinding after the
// block was made is invisible to it, a rebinding inside it is refused by
// the checker (src/syntax/syntax_test.go), its own variables are its own,
// and a lambda may call itself by the name it is assigned to.

func TestABlockCapturesValuesWhenItIsMade(t *testing.T) {
	out := runScript(t, `
a = 3
l = proc do a end
kept = { fn: l }
a = 4
puts l.call, kept[:fn].call
greeting = "hi"
routes = [-> { greeting }]
greeting = "bye"
puts routes[0].call, greeting
def make
  b = 5
  proc { b }
end
puts make.call
counter = 3.times.reduce(0) { |c, _| c + 1 }
puts counter
[1].each { |i| own = i + 10; puts own }
puts defined?(own).inspect
x = 2
puts [1].map { |i| y = x + i; y }.inspect, x
`)
	assert.Equals(t, out, "3\n3\nhi\nbye\n5\n3\n11\nnil\n[3]\n2\n")
}

func TestABlocksOwnRebindingLastsForItsCallOnly(t *testing.T) {
	// An assignment inside a block to a name it does not capture is the
	// call's own, so each call starts from the captured values.
	out := runScript(t, `
n = 1
l = -> { m = n + 1; m += 1; m }
puts l.call, l.call, n
def f
  s = 0
  3.times { |i| t = s + i; puts t }
  s
end
puts f
`)
	assert.Equals(t, out, "3\n3\n1\n0\n1\n2\n0\n")
}

func TestALambdaCallsItselfByItsName(t *testing.T) {
	out := runScript(t, `
fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }
puts fact.(5)
fib = proc { |n| n < 2 ? n : fib.call(n - 1) + fib.call(n - 2) }
puts fib.call(10)
walk = ->(n) { n == 0 ? [] : [n] + [n - 1].flat_map { |c| walk.(c) } }
p walk.(3)
x = x
p x
`)
	assert.Equals(t, out, "120\n55\n[3, 2, 1]\nnil\n")
}

func TestAForLoopAndARescueBindInTheScopeAroundThem(t *testing.T) {
	out := runScript(t, `
total = 0
for x in [1, 2, 3]
  total += x
end
p total, x
e = :none
[1].each do
  raise "inner"
rescue => e
  puts e.message
end
p e
`)
	assert.Equals(t, out, "6\n3\ninner\n:none\n")
}

func TestAsyncStartedInABlockIsAwaitedByItsMethod(t *testing.T) {
	out := runScript(t, `
include Cameo
class W
  def sq(i) = i * i
  def fan(list)
    list.each { |i| async(self, :sq, i) }
    await :all
  end
end
p W.new.fan([1, 2, 3])
`)
	assert.Equals(t, out, "[1, 4, 9]\n")
}

func TestTheCheckerRefusesARebindingInABlock(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "total = 0\n[1, 2].each { |x| total += x }\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "t.cameo:2: a block cannot rebind total, a variable of the scope around it: a block captures the values its variables have when it is made; bind a new name, or use reduce, each_with_object or while")
}
