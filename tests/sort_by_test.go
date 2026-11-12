package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// sort_by sorts by keys that are Integers, Floats, a mix of them, or
// anything else, and elements with one key keep their order whichever
// of these the keys are.
func TestSortByKeepsTheOrderOfElementsWithOneKey(t *testing.T) {
	out := runScript(t, `
srand(3)
a = 300.times.to_a.map { |i| [rand(7), i] }
by_int = a.sort_by { |x| x[0] }
p by_int.first(6), by_int == a.sort_by { |x| x[0].to_f }, by_int == a.sort_by { |x| x[0].odd? ? x[0].to_f : x[0] }
p by_int == a.sort_by { |x| x[0].to_s }, by_int == a.sort_by { |x| [x[0]] }
p [3, 1, 2].sort_by { |x| -x }, [].sort_by { |x| x }, %w[bb a ccc].sort_by(&:size)
p({ b: 1, a: 2 }.sort_by { |k, v| v })
p [2**70, 1, 2.5, 2**60].sort_by { |x| x }
p [1.5, -0.0, 0.0, -1].sort_by { |x| x }
r = begin
  [1, "a"].sort_by { |x| x }
rescue => e
  e.class
end
p r
p (1..5).sort_by { |x| -x }, [3, 1].sort_by { |x| break :out }
`)
	assert.Equals(t, out, `[[0, 1], [0, 4], [0, 5], [0, 6], [0, 17], [0, 22]]
true
true
true
true
[3, 2, 1]
[]
["a", "bb", "ccc"]
[[:b, 1], [:a, 2]]
[1, 2.5, 1152921504606846976, 1180591620717411303424]
[-1, -0.0, 0.0, 1.5]
ArgumentError
[5, 4, 3, 2, 1]
:out
`)
}
