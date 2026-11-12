package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A Hash's each and each_pair call the block with one value, the pair
// of the key and the value, as Ruby's do. A block with a parameter for
// each takes them apart; one with a single parameter, a splat, or a
// method given as the block gets the pair.
func TestHashEachCallsItsBlockWithThePair(t *testing.T) {
	out := runScript(t, `
h = { a: 1, b: 2 }
h.each { |k, v| p [k, v] }
{ a: 1 }.each { |k, v| p k, v }
h.each { |pair| p pair }
h.each_pair { |pair| p pair }
h.each { |*all| p all }
h.each { |k, v, more| p [k, v, more] }
h.each { |k, v = 5| p [k, v] }
h.each { |(k, v)| p [v, k] }
p h.each(&:first)
p h.each { }.size
h.each_with_index { |pair, i| p [pair, i] }
h.each_with_index { |(k, v), i| p [k, v, i] }
p h.map { |pair| pair }, h.map { |k, v| [v, k] }
p h.select { |k| k == :a }, h.select { |k, v| v == 2 }
`)
	assert.Equals(t, out, `[:a, 1]
[:b, 2]
:a
1
[:a, 1]
[:b, 2]
[:a, 1]
[:b, 2]
[[:a, 1]]
[[:b, 2]]
[:a, 1, nil]
[:b, 2, nil]
[:a, 1]
[:b, 2]
[1, :a]
[2, :b]
{a: 1, b: 2}
2
[[:a, 1], 0]
[[:b, 2], 1]
[:a, 1, 0]
[:b, 2, 1]
[[:a, 1], [:b, 2]]
[[1, :a], [2, :b]]
{a: 1}
{b: 2}
`)
}

// each_slice, each_cons and each_entry return the receiver when they
// are given a block, as Ruby's do since 3.1.
func TestEachSliceConsAndEntryReturnTheReceiver(t *testing.T) {
	out := runScript(t, `
p [1, 2, 3].each_slice(2) { }, [1, 2, 3].each_cons(2) { }, [1, 2].each_entry { }
p (1..3).each_slice(2) { }, { a: 1 }.each_cons(1) { }, { a: 1 }.each_entry { |x| p x }
`)
	assert.Equals(t, out, "[1, 2, 3]\n[1, 2, 3]\n[1, 2]\n[:a, 1]\n1..3\n{a: 1}\n{a: 1}\n")
}
