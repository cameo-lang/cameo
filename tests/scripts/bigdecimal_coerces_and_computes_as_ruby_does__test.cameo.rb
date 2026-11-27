# bigdecimal: testcase to demonstrate that big decimal coerces and computes as
# Ruby does
#
# Ruby's coerce protocol holds everywhere a number meets a value of another
# class: == asks the value, <=> and the relations go through its coerce,
# structural equality and remainder included; a BigDecimal follows Ruby for a
# zero base, an infinite divisor, a signed zero, and BigMath of an infinity; a
# comparison across a gap of exponents scales nothing. The answers are Ruby
# 3.4.1's.
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "bigdecimal"
require "bigdecimal/util"
require "bigdecimal/math"
p "-".to_d, "-".to_d.sign, BigDecimal("0") ** 0.5, BigDecimal("0") ** BigDecimal("2.5"), BigDecimal("0") ** -0.5
p BigDecimal("5") % BigDecimal("Infinity"), BigDecimal("5").remainder(BigDecimal("Infinity")), BigDecimal("5").divmod(BigDecimal("Infinity")), BigDecimal("-5") % BigDecimal("Infinity"), BigDecimal("Infinity") % 5
r = BigDecimal("1e100000") + 1
p r.exponent, r.precision, (BigDecimal("1e1000000000") > 1), (BigDecimal("1e1000000000") <=> BigDecimal("1e999999999"))
class C2
  def coerce(o)
    [o.to_f, 2.0]
  end
  def <=>(o)
    0
  end
end
p(1 <=> C2.new, 1 < C2.new)
class W
  def ==(o)
    true
  end
end
p 1 == W.new, 1.5 == W.new, 1 != BigDecimal("1"), [1].include?(BigDecimal("1")), 7.remainder(BigDecimal("2")), 7.5.remainder(BigDecimal("2")), Integer(BigDecimal("1.5"))
p [1, 2] == [BigDecimal("1"), 2], (1 === BigDecimal("1")), {a: 1} == {a: BigDecimal("1")}, [1, 2].index(BigDecimal("2")), BigDecimal("0") * -1, BigDecimal("2") * BigDecimal("-0")
p BigMath.log(BigDecimal("Infinity"), 10), BigMath.sqrt(BigDecimal::INFINITY, 10), BigMath.exp(BigDecimal("Infinity"), 10), BigMath.exp(BigDecimal("-Infinity"), 10), BigMath.atan(BigDecimal("Infinity"), 10), BigMath.atan(BigDecimal("-Infinity"), 10), BigMath.sin(BigDecimal("Infinity"), 10)
p BigDecimal("1").hash == BigDecimal("1.0").hash, {BigDecimal("1") => :a}[BigDecimal("1.0")]

__END__
-0.0
-1
0.0
0.0
Infinity
0.5e1
0.5e1
[0.0, 0.5e1]
-0.5e1
NaN
100001
100001
true
1
-1
true
true
true
false
true
0.1e1
0.15e1
1
true
true
true
1
-0.0
-0.0
Infinity
Infinity
Infinity
0.0
0.1570796327e1
-0.1570796327e1
NaN
true
:a
