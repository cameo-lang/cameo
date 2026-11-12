package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/object"
)

// A builtin called with its arguments where they are on the stack
// returns what it returns when it is called with a slice of its own: []
// and key? of a hash, of one with a default block too, [] and at of an
// array, builtins without arguments, values read by several such calls
// in one expression, and next to them the calls the general call still
// makes: new with a script's initialize, a proc and a lambda, a method
// of the script, a wrong number of arguments, a private function called
// on a receiver, and an argument of the wrong type.
func TestBuiltinsCalledWithBorrowedArguments(t *testing.T) {
	expectOutput(t, `class Point
  def initialize
    print "made "
  end
end
h = { 1 => "one", "two" => 2, :three => [3] }
l = [10, 20, 30]
counted = Hash.new { |hash, k| k.to_s * 2 }
p h[1], h["two"], h[:three], h[4], h.key?(1), h.include?("no"), h.member?(:three), h.has_key?(2)
p l[0], l[-1], l[5], l[1, 2], l[0..1], l.at(2), l.size, l.length, l.first, l.last, l.empty?
p [h[1], l[1], h["two"] + l[2], h[:three][0]]
p counted[21], counted["ab"], "abc".size, :sym.to_s, 12.to_s, nil.to_a, 3.zero?, "x".upcase
pr = proc { 5 }
lam = ->(n) { n == 0 ? :done : lam.call(n - 1) }
p pr.call, lam.call(3), Object.new.class, Point.new.class
def helper = 7
p helper, self.class
begin
  l.at
rescue ArgumentError => e
  p e.message
end
begin
  1.puts
rescue NoMethodError => e
  p e.message
end
begin
  l["x"]
rescue TypeError => e
  p e.message
end
`, `"one"
2
[3]
nil
true
false
true
false
10
30
nil
[20, 30]
[10, 20]
30
3
3
10
30
false
["one", 20, 32, 3]
"2121"
"abab"
3
"sym"
"12"
[]
false
"X"
made 5
:done
Object
Point
7
Object
"wrong number of arguments (given 0, expected 1)"
"private method 'puts' called for an instance of Integer"
"no implicit conversion of String into Integer"
`)
}

// The builtins marked as keeping no slice of their arguments are the
// ones the evaluator calls so, and a method a script defines is none.
func TestBorrowingBuiltinsAreMarked(t *testing.T) {
	New(nil, nil)
	for _, name := range []string{"[]", "key?", "has_key?", "include?", "member?"} {
		assert.True(t, object.MapClass.Methods[name].Borrows, "Hash#"+name)
	}
	for _, name := range []string{"[]", "at"} {
		assert.True(t, object.ListClass.Methods[name].Borrows, "Array#"+name)
	}
	for _, name := range []string{"[]", "split", "include?", "%"} {
		assert.True(t, object.StringClass.Methods[name].Borrows, "String#"+name)
	}
	assert.True(t, object.Enumerable.Methods["first"].Borrows)
	// A method that makes an enumerator of its arguments keeps them.
	assert.True(t, !object.Enumerable.Methods["each_slice"].Borrows)
	assert.True(t, !object.StringClass.Methods["sub"].Borrows)
}

// The builtins of hashes, arrays, enumerables and strings that are
// called with their arguments on the stack return what they did: with
// one argument and with several, a list they make of their arguments'
// values after later calls used the stack again, and the errors they
// raise.
func TestMoreBuiltinsCalledWithBorrowedArguments(t *testing.T) {
	expectOutput(t, `h = { a: 1, "b" => [2, { c: 3 }], 4 => :four }
l = [5, [6, [7]], "x", :y, 8]
s = "hello, world"
keep = []
keep += [h.fetch(:a), h.fetch(:zz, 0), h.key(1), h.dig("b", 1, :c), h.values_at(:a, 4, :no)]
keep += [l.fetch(1), l.fetch(9, :none), l.dig(1, 1, 0), l.index("x"), l.last(2), l.last, l.join("-"), l.values_at(0, -1, 9)]
keep += [l.include?(:y), l.include?(0), l.first(2), l.first, (1..5).include?(3), (1..5).first(2)]
keep += [s[0], s[1, 3], s[0..4], s[-5..], s["wor"], s[/l+/], s.slice(7, 5), s + "!", s * 2, "%05d|%s" % [42, :z], "%x" % 255]
keep += [s == "hello, world", s <=> "help", s =~ /wor/, s.tr("lo", "01"), s.split(", "), s.split, s.split("", 3)]
keep += [s.include?("lo, w"), s.start_with?("he", "x"), s.end_with?("ld"), s.delete_prefix("hello"), s.delete_suffix("world"), s.count("lo")]
p keep
# The values of earlier calls are as they were after later calls reused the stack.
a = l.values_at(0, 4)
b = h.values_at(:a, 4)
c = l.first(3)
p a, b, c, [l.last(2), l.values_at(1), h.values_at("b")]
begin
  h.fetch(:missing)
rescue KeyError => e
  p e.message
end
begin
  s + 1
rescue TypeError => e
  p e.message
end
`, `[1, 0, :a, 3, [1, :four, nil], [6, [7]], :none, 7, 2, [:y, 8], 8, "5-6-7-x-y-8", [5, 8, nil], true, false, [5, [6, [7]]], 5, true, [1, 2], "h", "ell", "hello", "world", "wor", "ll", "world", "hello, world!", "hello, worldhello, world", "00042|z", "ff", true, -1, 7, "he001, w1r0d", ["hello", "world"], ["hello,", "world"], ["h", "e", "llo, world"], true, true, true, ", world", "hello, ", 5]
[5, 8]
[1, :four]
[5, [6, [7]], "x"]
[[:y, 8], [[6, [7]]], [[2, {c: 3}]]]
"key not found: :missing"
"no implicit conversion of Integer into String"
`)
}
