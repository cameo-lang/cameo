package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// The Syms of the operators are in the order of the BINARY_ instructions,
// which is how an instruction finds its operator.
func TestBinaryInstructionsAndSymsAreInOneOrder(t *testing.T) {
	assert.Equals(t, len(binaryNames), int(cameopb.Op_BINARY_NE-cameopb.Op_BINARY_PLUS)+1)
	for i, name := range binaryNames {
		assert.Equals(t, object.Intern(name), object.SymAdd+object.Sym(i))
	}
	assert.Equals(t, cameopb.Op_BINARY_MODULO-cameopb.Op_BINARY_PLUS, cameopb.Op(4))
	assert.Equals(t, binaryNames[cameopb.Op_BINARY_GE-cameopb.Op_BINARY_PLUS], ">=")
}

// The operators compute on two integers and on two floats, and call the
// method on anything else: a big result, a mixed pair, a string, a class
// that defines the operator, a zero divisor.
func TestBinaryOperators(t *testing.T) {
	expectOutput(t, `class V
  int :n
  def +(o) = V.new(n: n + o.n)
  def ==(o) = n == o.n
  def <(o) = n < o.n
end
p 7 + 2, 7 - 2, 7 * 2, 7 / 2, -7 / 2, 7 % 3, -7 % 3
p 1 < 2, 2 <= 2, 1 > 2, 2 >= 3, 2 == 2, 2 != 2
p 1.5 + 2.5, 1.5 - 2.0, 1.5 * 2.0, 3.0 / 2.0, 1.5 < 2.5, 2.5 == 2.5, 2.5 != 2.5
p 9223372036854775807 + 1, 4611686018427387904 * 4, -9223372036854775807 - 2
p 1 + 2.5, 2.5 > 1, 1 == 1.0, 7 % 2.0
p "a" + "b", "a" < "b", "a" == "a", [1] + [2], [1] == [1], :a != :b
p (V.new(n: 1) + V.new(n: 2)).n, V.new(n: 1) == V.new(n: 1), V.new(n: 3) < V.new(n: 2)
begin
  1 / 0
rescue ZeroDivisionError => e
  puts e.message
end
begin
  1 + nil
rescue TypeError => e
  puts e.message
end
`, `9
5
14
3
-4
1
2
true
true
false
false
true
false
4.0
-0.5
3.0
1.5
true
true
false
9223372036854775808
18446744073709551616
-9223372036854775809
3.5
true
true
1.0
"ab"
true
true
[1, 2]
true
true
3
true
false
divided by 0
nil can't be coerced into Integer
`)
}
