package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A proc is a closure over the binding it was made in, as in Ruby: it reads
// the variables of that binding when it is called, not when it was made, so a
// rebinding after the proc was made shows in the next call. Keeping the proc
// in a Hash, an Array, a method argument or a method's return value changes
// nothing: the binding travels with the proc.

func TestAProcReadsTheBindingItWasMadeIn(t *testing.T) {
	out := runScript(t, `
a = 3
l = proc do a end
puts l.call
kept = { fn: l }
list = [l]
def run(fn) = fn.call
puts kept[:fn].call, list[0].call, run(l)
a = 4
puts l.call, kept[:fn].call
def make
  b = 5
  proc { b }
end
m = make
puts m.call
f = -> { a + m.call }
puts f.call
`)
	assert.Equals(t, out, "3\n3\n3\n3\n4\n4\n5\n9\n")
}
