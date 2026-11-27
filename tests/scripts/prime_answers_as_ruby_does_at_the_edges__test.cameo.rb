# prime: testcase to demonstrate that prime answers as Ruby does at the edges
#
# include? is false for what is not an Integer, a blockless each's Enumerator
# returns the block's last value from each, a negative exponent is refused,
# the messages interpolate the value as prime's do, and the sieve counts to
# two million with its base primes alone.
#
# The expected values are Ruby's, with prime 0.1.3.
#
# (Introduced in PR #276. expected output follows the __END__ separator.)
require "prime"
p Prime.include?(7.0), Prime.include?("x"), Prime.include?(nil), Prime.include?(Comparable), Prime.include?(7)
p Prime.each(10).each { |x| x }, Prime.each(2**70).first(3), Prime.each(1.5).to_a, Prime.each(-1).to_a
begin; Prime.int_from_prime_division([[2, -1]]); rescue ArgumentError => e; puts e.message; end
begin; Prime.prime?("x"); rescue ArgumentError => e; puts e.message.inspect; end
begin; Prime.prime?(nil); rescue ArgumentError => e; puts e.message.inspect; end
p Prime.each(2_000_000).count, Prime.each(1_100_000).to_a.last

__END__
false
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
