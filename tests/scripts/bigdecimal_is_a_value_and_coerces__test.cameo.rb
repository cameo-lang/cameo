# bigdecimal: testcase to demonstrate that big decimal is a value and coerces
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "bigdecimal"
a = BigDecimal("1.23")
p Set[a, BigDecimal("1.230"), BigDecimal("1.24")].size, {a => :x}[BigDecimal("1.230")], a.frozen?
p 1 + a, 2.5 * a, 10 / a, 1 == a, 1.23 == a, 1 < a, [a, 1, 1.5].sort, [a, 1, 1.5].max, 1.clamp(a, 2), a.clamp(1, 2), 2.between?(a, 3)
p Integer === a, Numeric === a, a.is_a?(Comparable), a.class.ancestors.take(3)

__END__
2
:x
true
0.223e1
0.3075e1
0.8130081300813008130081300813008130082e1
false
true
true
[1, 0.123e1, 1.5]
1.5
0.123e1
0.123e1
true
false
true
true
[BigDecimal, Numeric, Comparable]
