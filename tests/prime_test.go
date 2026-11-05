package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby's, with prime 0.1.3.

func TestPrimeFollowsRubys(t *testing.T) {
	out := runScript(t, `require "prime"
p Prime.first(10), Prime.take(5), Prime.each(30).to_a, Prime.each(2).to_a, Prime.each(1).to_a, Prime.each(-5).to_a, Prime.each(3.5).to_a
p [2, 1, 0, -7, 97, 100, 2**31 - 1, 2**61 - 1, 10**18 + 9, 1_000_003, 561].map { |n| Prime.prime?(n) }, [7, 1, 0, -3, 91, 2**64 + 13].map(&:prime?)
p Prime.prime_division(360), Prime.prime_division(1), Prime.prime_division(-12), Prime.prime_division(2), 360.prime_division, Prime.prime_division(2**32 + 1), Prime.prime_division(10**12 + 39), Prime.prime_division(1_000_000_007 * 1_000_000_009), (2**70 + 25).prime_division
p Prime.int_from_prime_division([[2, 3], [3, 2], [5, 1]]), Integer.from_prime_division([[2, 3], [3, 2], [5, 1]]), Prime.int_from_prime_division([]), Prime.int_from_prime_division([[-1, 1], [2, 2]])
p Prime.each.take(5), Prime.each.lazy.select { |x| x % 10 == 3 }.first(4), Prime.lazy.map { |x| x * 2 }.first(3), Prime.each(10) { |x| x }, Prime.include?(7), Prime.include?(8), Prime.instance.class, Prime.each.class
p Integer.each_prime(10).to_a, Prime.each(10).map { |x| x * x }, Prime.each(100).count, Prime.each(10).include?(5), Prime.each(1000).to_a.last(3), Prime.each(20).each_slice(3).to_a, Prime.each.first(3), Prime.each(30).select(&:even?), Prime.each(10).size, Prime::VERSION
begin; Prime.prime_division(0); rescue ZeroDivisionError => e; p e.class; end
begin; Prime.each("x").to_a; rescue ArgumentError => e; puts e.message; end
begin; Prime.prime?(7.0); rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `[2, 3, 5, 7, 11, 13, 17, 19, 23, 29]
[2, 3, 5, 7, 11]
[2, 3, 5, 7, 11, 13, 17, 19, 23, 29]
[2]
[]
[]
[2, 3]
[true, false, false, false, true, false, true, true, true, true, false]
[true, false, false, false, false, true]
[[2, 3], [3, 2], [5, 1]]
[]
[[-1, 1], [2, 2], [3, 1]]
[[2, 1]]
[[2, 3], [3, 2], [5, 1]]
[[641, 1], [6700417, 1]]
[[1000000000039, 1]]
[[1000000007, 1], [1000000009, 1]]
[[1180591620717411303449, 1]]
360
360
1
-4
[2, 3, 5, 7, 11]
[3, 13, 23, 43]
[4, 6, 10]
7
true
false
Prime
Enumerator
[2, 3, 5, 7]
[4, 9, 25, 49]
25
true
[983, 991, 997]
[[2, 3, 5], [7, 11, 13], [17, 19]]
[2, 3, 5]
[2]
Infinity
"0.1.3"
ZeroDivisionError
comparison of Integer with String failed
Expected an integer, got 7.0
`)
}

// include? is false for what is not an Integer, a blockless each's
// Enumerator returns the block's last value from each, a negative
// exponent is refused, the messages interpolate the value as prime's do,
// and the sieve counts to two million with its base primes alone.
func TestPrimeAnswersAsRubyDoesAtTheEdges(t *testing.T) {
	out := runScript(t, `require "prime"
p Prime.include?(7.0), Prime.include?("x"), Prime.include?(nil), Prime.include?(Comparable), Prime.include?(7)
p Prime.each(10).each { |x| x }, Prime.each(2**70).first(3), Prime.each(1.5).to_a, Prime.each(-1).to_a
begin; Prime.int_from_prime_division([[2, -1]]); rescue ArgumentError => e; puts e.message; end
begin; Prime.prime?("x"); rescue ArgumentError => e; puts e.message.inspect; end
begin; Prime.prime?(nil); rescue ArgumentError => e; puts e.message.inspect; end
p Prime.each(2_000_000).count, Prime.each(1_100_000).to_a.last
`)
	assert.Equals(t, out, `false
false
false
false
true
7
[2, 3, 5]
[]
[]
a negative exponent, -1 for 2, makes a Rational, which cameo has not
"Expected an integer, got x"
"Expected an integer, got "
148933
1099997
`)
}
