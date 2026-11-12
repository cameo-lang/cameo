package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A list a script appends to is a new list each time, whichever of <<,
// push and append makes it, and the lists made before stay what they
// were; map and the builtins that gather return what the block returned.
func TestAppendingMakesANewListEachTime(t *testing.T) {
	out := runScript(t, `
a = [1, 2]
b = a
a = a << 3
c = a
a = a.push(4, 5)
d = a.append(6)
e = c.append(7)
f = c.append(8)
puts b.inspect, c.inspect, a.inspect, d.inspect, e.inspect, f.inspect
out = []
i = 0
while i < 100
  out = out << i
  i += 1
end
puts out.size, out.sum, out.last
puts [1, 2, 3].map { |x| x * 2 }.inspect, [].map { |x| x }.inspect
m = [1, 2, 3].map { |x| x + 1 }
n = m.append(9)
puts m.inspect, n.inspect
puts [1, 2, 3, 4].select { |x| x.even? }.inspect, [1, 2, 3].take_while { |x| x < 3 }.inspect
puts({ a: 1 }.map { |k, v| [k, v] }.inspect, [1, 2].flat_map { |x| [x, x] }.inspect)
`)
	assert.Equals(t, out, `[1, 2]
[1, 2, 3]
[1, 2, 3, 4, 5]
[1, 2, 3, 4, 5, 6]
[1, 2, 3, 7]
[1, 2, 3, 8]
100
4950
99
[2, 4, 6]
[]
[2, 3, 4]
[2, 3, 4, 9]
[2, 4]
[1, 2]
[[:a, 1]]
[1, 1, 2, 2]
`)
}
