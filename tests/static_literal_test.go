package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A list or a map written with literals alone is one value, made when
// its code is lowered, and every run of the code has it. Nothing a
// script does to what it holds changes the literal: the next run
// starts from the same list and the same map. One with a variable, a
// splat or a list inside it is made each time, as before.
func TestAListOrMapOfLiteralsIsTheSameEachTime(t *testing.T) {
	out := runScript(t, `
def fresh
  h = { a: 1, "b" => 2.5, 3 => nil, true => :t }
  l = [1, "two", :three, 4.0, nil, false]
  h[:more] = 9
  l = l << 7
  [h, l]
end
p fresh, fresh
3.times do |i|
  h = { a: 1, b: 2 }
  h[i] = i
  l = [1, 2]
  l = l.push(i)
  p h, l
end
p({ a: 1, a: 2 }, { a: 1, **{ b: 2 } }, { a: 1, **{} }, {}, [], [[1, 2], [3]], { a: [1], b: { c: 2 } })
x = 5
p [1, x, 3], { a: x, b: 2 }, { x => 1, y: 2 }, [1, *[2, 3]], [:a, :b].include?(:a)
a = [1, 2, 3]
b = a
a = a + [4]
p a, b, [1, 2, 3].map { |v| v * 2 }, [3, 1, 2].sort, { b: 1, a: 2 }.sort.to_h, { a: 1 }.merge(b: 2)
i = 0
while i < 2
  p [i, [1, 2], { k: "v" }]
  i += 1
end
def kw(**opts) = opts
p kw(a: 1, b: 2), kw
c = case 2 when 1, 2 then [:one, :two] else [] end
p c, [1, 2].frozen?, { a: 1 }.frozen?, [1, 2] == [1, 2], { a: 1 }.eql?({ a: 1 }), [1, 2].hash == [1, 2].hash
`)
	assert.Equals(t, out, `[{a: 1, "b" => 2.5, 3 => nil, true => :t, more: 9}, [1, "two", :three, 4.0, nil, false, 7]]
[{a: 1, "b" => 2.5, 3 => nil, true => :t, more: 9}, [1, "two", :three, 4.0, nil, false, 7]]
{a: 1, b: 2, 0 => 0}
[1, 2, 0]
{a: 1, b: 2, 1 => 1}
[1, 2, 1]
{a: 1, b: 2, 2 => 2}
[1, 2, 2]
{a: 2}
{a: 1, b: 2}
{a: 1}
{}
[]
[[1, 2], [3]]
{a: [1], b: {c: 2}}
[1, 5, 3]
{a: 5, b: 2}
{5 => 1, y: 2}
[1, 2, 3]
true
[1, 2, 3, 4]
[1, 2, 3]
[2, 4, 6]
[1, 2, 3]
{a: 2, b: 1}
{a: 1, b: 2}
[0, [1, 2], {k: "v"}]
[1, [1, 2], {k: "v"}]
{a: 1, b: 2}
{}
[:one, :two]
true
true
true
true
true
`)
}
