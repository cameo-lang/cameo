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
	assert.True(t, !object.ListClass.Methods["fetch"].Borrows)
	assert.True(t, !object.MapClass.Methods["fetch"].Borrows)
}
