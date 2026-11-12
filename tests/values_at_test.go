package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// values_at takes the elements of a range in order and nil for what is
// past the end, as many as the range is long, among indexes and other
// ranges.
func TestValuesAtTakesRanges(t *testing.T) {
	out := runScript(t, `
a = [10, 20, 30, 40, 50]
p a.values_at(1..3), a.values_at(1...3), a.values_at(3..7), a.values_at(5..6), a.values_at(7..8)
p a.values_at(-2..), a.values_at(..1), a.values_at(-3..-2), a.values_at(2..1), a.values_at(0, 1..2, -1, 4..5)
p a.values_at(0..4), a.values_at, [].values_at(0..1), a.values_at(1..3).size
r = begin
  a.values_at(-9..1)
rescue RangeError => e
  e.message
end
p r
b = a.values_at(0..4)
c = b + [60]
p a, b, c
`)
	assert.Equals(t, out, `[20, 30, 40]
[20, 30]
[40, 50, nil, nil, nil]
[nil, nil]
[nil, nil]
[40, 50]
[10, 20]
[30, 40]
[]
[10, 20, 30, 50, 50, nil]
[10, 20, 30, 40, 50]
[]
[nil, nil]
3
"-9..1 out of range"
[10, 20, 30, 40, 50]
[10, 20, 30, 40, 50]
[10, 20, 30, 40, 50, 60]
`)
}
