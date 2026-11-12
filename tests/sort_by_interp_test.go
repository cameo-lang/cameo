package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// sort_by sorts by keys that are Integers, Floats, a mix of them, or
// anything else, and elements with one key keep their order whichever
// of these the keys are. A string with interpolation is its parts,
// whether a part is a string with parts of its own, an object with a
// to_s, or left by next; a string it is made of is unchanged. A call
// that is a statement runs as any call.
func TestSortByInterpolationAndStatementCalls(t *testing.T) {
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
x = "é".b
r = begin
  "ü#{x}"
rescue => e
  e.class
end
p r
p "#{}", "#{"only"}", "a#{"b#{"c#{1}d"}e"}f#{2}", "#{"x" * 40}#{"y" * 40}#{"z" * 40}".size
p [1, 2, 3].map { |i| "v#{i}-#{i * 2}" }
p [1, 2].map { |i| "a#{next "n" if i == 1}b#{i}" }
class Pt
  def to_s = "pt"
end
p "#{Pt.new}|#{Pt.new}", "#{1}#{Pt.new}#{2}#{nil}#{:s}#{1.5}"
s = "kept"
t = "#{s}"
u = "#{s}!"
p s, t, u, "#{t}#{u}"
def twice(v) = v * 2
def noisy(v)
  puts "noisy #{v}"
  v
end
twice 2
noisy 3
noisy(4)
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
Encoding::CompatibilityError
""
"only"
"abc1def2"
120
["v1-2", "v2-4", "v3-6"]
["n", "ab2"]
"pt|pt"
"1pt2s1.5"
"kept"
"kept"
"kept!"
"keptkept!"
noisy 3
noisy 4
`)
}

// A call that drops the copy force_encoding returns is still refused.
func TestADroppedCopyIsStillRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "def f(s)\n  s.force_encoding(\"BINARY\")\n  1\nend\np f(\"abc\")\n", &stdout, &stderr)
	assert.True(t, err != nil && strings.Contains(err.Error(), "force_encoding returns a copy"))
}
