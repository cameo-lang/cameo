# bigdecimal: testcase to demonstrate that big decimal matches Ruby
#
# The expected values are Ruby 3.3's, with bigdecimal 3.1.5, but where the
# probe says otherwise: sqrt and BigMath give the digits asked for (arith2,
# props, math), BigDecimal::VERSION is 3.1.8, Ruby 3.4's, and a String is
# frozen (misc).
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "bigdecimal"
require "bigdecimal/util"
require "bigdecimal/math"
def t(label)
  r = yield
  puts "#{label}: #{r.inspect}"
rescue Exception => e
  puts "#{label}: #{e.class}: #{e.message}"
end
a = BigDecimal("1.23")
b = BigDecimal("4.5")
t("new") { [a, b, BigDecimal("1"), BigDecimal("-0.001"), BigDecimal("1e10"), BigDecimal("1.5e-3"), BigDecimal("0"), BigDecimal("-0"), BigDecimal(" 1.5 "), BigDecimal("1_000.5"), BigDecimal(3), BigDecimal(-7), BigDecimal(2**70), BigDecimal(1.5, 3), BigDecimal(1.0/3, 10), BigDecimal(a), BigDecimal("Infinity"), BigDecimal("-Infinity"), BigDecimal("NaN"), BigDecimal("+1.5"), BigDecimal(".5"), BigDecimal("1E+3"), BigDecimal("0.00"), BigDecimal("123456789012345678901234567890.123456789")] }
t("new_err") { BigDecimal("abc") }
t("new_err2") { BigDecimal("1.2.3") }
t("new_err3") { BigDecimal(1.5) }
t("new_err4") { BigDecimal(nil) }
t("new_err5") { BigDecimal("") }
t("new_err6") { BigDecimal("1 2") }
t("new_err7") { BigDecimal("0x10") }
t("new_err8") { BigDecimal("1e") }
t("new_exc") { [BigDecimal("abc", exception: false), BigDecimal(nil, exception: false)] }
t("to_s") { [a.to_s, b.to_s, BigDecimal("100").to_s, BigDecimal("0.001").to_s, BigDecimal("-1.5").to_s, BigDecimal("0").to_s, BigDecimal("-0").to_s, BigDecimal("1e20").to_s, BigDecimal("1.5e-7").to_s, BigDecimal("Infinity").to_s, BigDecimal("NaN").to_s, BigDecimal("-Infinity").to_s, BigDecimal("123.456").to_s("F"), BigDecimal("123.456").to_s("+F"), BigDecimal("123.456").to_s(" F"), BigDecimal("100").to_s("F"), BigDecimal("0.001").to_s("F"), BigDecimal("-0.5").to_s("F"), BigDecimal("0").to_s("F"), BigDecimal("1e20").to_s("F"), BigDecimal("1234567.891").to_s("3F"), BigDecimal("1234567.891").to_s("3E"), BigDecimal("1234567.891").to_s(3), BigDecimal("1234567.891").to_s("E"), BigDecimal("1234567.891").to_s("+"), BigDecimal("1234567.891").to_s(" ")] }
t("inspect") { [a.inspect, BigDecimal("100").inspect, BigDecimal("0.001").inspect, BigDecimal("1e20").inspect, BigDecimal("-0").inspect] }
t("arith") { [a + b, a - b, a * b, a / b, b / a, a % b, b % a, -a, +a, a ** 2, a ** -1, b ** 0.5, a.abs, (-a).abs, a + 1, a - 1.5, a * 2, a / 2, 1 + a, 2 * a, 1.5 - a, 10 / a, a.add(b, 2), a.sub(b, 2), a.mult(b, 3), a.div(b, 5), a.div(b), b.div(a), a.quo(b), a.divmod(b), b.divmod(a), a.remainder(b), b.modulo(a), a.fdiv(b)] }
t("arith2") { [BigDecimal("1") / BigDecimal("3"), BigDecimal("2") / BigDecimal("3"), BigDecimal("1") / BigDecimal("7"), BigDecimal("10") / BigDecimal("4"), BigDecimal("1") / 3, BigDecimal("0.1") + BigDecimal("0.2"), BigDecimal("0.1") * 3, BigDecimal("1e-20") + 1, BigDecimal("12345678901234567890") * BigDecimal("98765432109876543210"), BigDecimal("1") / BigDecimal("3") * 3, BigDecimal("2").sqrt(10), BigDecimal("2").sqrt(30), BigDecimal("16").sqrt(5), BigDecimal("0").sqrt(5), BigDecimal("1") / 0, BigDecimal("-1") / 0, BigDecimal("0") / 0] } 
t("arith3") { BigDecimal("1") / BigDecimal("0") }
t("arith4") { BigDecimal("1").div(0) }
t("arith5") { BigDecimal("-1").sqrt(5) }
t("arith6") { [BigDecimal("1") / BigDecimal("3.0"), (BigDecimal("1") / BigDecimal("3")).precision, (BigDecimal("1") / BigDecimal("3")).scale, (BigDecimal("1") / BigDecimal("3")).n_significant_digits, BigDecimal("1").div(BigDecimal("3"), 50), BigDecimal("1").div(3, 0), BigDecimal("22").div(BigDecimal("7"), 3)] }
t("cmp") { [a < b, a > b, a <= a, a >= b, a == BigDecimal("1.23"), a == 1.23, a == BigDecimal("1.230"), a.eql?(BigDecimal("1.230")), a <=> b, b <=> a, a <=> a, a <=> 1, a <=> "x", a == "1.23", a.hash == BigDecimal("1.230").hash, [b, a].sort, [a, b].max, a.between?(1, 2), a.clamp(2, 3), BigDecimal("1") == 1, BigDecimal("1") == 1.0, 1 == BigDecimal("1"), 1.0 == BigDecimal("1"), BigDecimal("NaN") == BigDecimal("NaN"), BigDecimal("Infinity") > 10**100, BigDecimal("1.5") > 1, BigDecimal("1.5") < 2.0] }
t("cmp2") { a < "x" }
t("round") { [a.round, a.round(1), a.round(5), b.round, BigDecimal("2.5").round, BigDecimal("-2.5").round, BigDecimal("3.5").round, BigDecimal("2.5").round(0, :half_even), BigDecimal("3.5").round(0, :half_even), BigDecimal("2.5").round(0, :half_down), BigDecimal("2.5").round(0, :down), BigDecimal("2.5").round(0, :up), BigDecimal("2.5").round(0, :ceiling), BigDecimal("-2.5").round(0, :floor), BigDecimal("2.4").round(0, :ceil), BigDecimal("2.6").round(0, :truncate), BigDecimal("1.23456").round(3), BigDecimal("1.23456").round(3, BigDecimal::ROUND_DOWN), BigDecimal("1.23456").round(-1), BigDecimal("1234.5").round(-2), a.round.class, a.round(1).class, a.floor, a.ceil, a.truncate, a.fix, a.frac, (-a).floor, (-a).ceil, (-a).truncate, (-a).frac, a.floor(1), a.ceil(1), a.truncate(1), BigDecimal("1.99").floor(1), BigDecimal("1.91").ceil(1), a.floor.class, a.floor(1).class, b.to_i, a.to_i, (-a).to_i, a.to_int, a.truncate.class] }
t("round2") { BigDecimal("2.5").round(0, :bogus) }
t("round3") { [BigDecimal("2.5").round(0, 1), BigDecimal("2.5").round(0, 2), BigDecimal("2.5").round(0, 3), BigDecimal("2.5").round(0, 4), BigDecimal("2.5").round(0, 5), BigDecimal("2.5").round(0, 6), BigDecimal("2.5").round(0, 7), BigDecimal("2.5").round(0, "half_even"), BigDecimal("Infinity").round, BigDecimal("NaN").round] }
t("conv") { [a.to_f, a.to_i, a.to_s, a.to_d, b.to_f, BigDecimal("1e30").to_f, BigDecimal("1e-400").to_f, BigDecimal("1e400").to_f, BigDecimal("Infinity").to_f, BigDecimal("NaN").to_f.nan?] }
t("conv3") { BigDecimal("Infinity").to_i } 
t("conv2") { BigDecimal("NaN").to_i }
t("util") { ["1.5".to_d, "abc".to_d, "".to_d, 3.to_d, 1.5.to_d, 0.1.to_d, (1.0/3).to_d, 1.5.to_d(2), 0.1.to_d(5), nil.to_d, a.to_d, "1e3".to_d, " 2.5x".to_d, 1e20.to_d, (2**70).to_d, "1.5".to_d.class, 1.to_d.class, "0.1".to_d + "0.2".to_d] }
t("util2") { [a.to_digits, BigDecimal("100").to_digits, BigDecimal("0.001").to_digits, BigDecimal("-1.5").to_digits, BigDecimal("1e20").to_digits, BigDecimal("1").to_digits] }
t("props") { [a.precision, a.scale, a.precision_scale, a.n_significant_digits, BigDecimal("100").precision, BigDecimal("100").scale, BigDecimal("100").n_significant_digits, BigDecimal("0.001").precision, BigDecimal("0.001").scale, BigDecimal("0").precision, BigDecimal("0").scale, BigDecimal("123.456").precision, BigDecimal("123.456").scale, BigDecimal("1e20").precision, BigDecimal("1e-20").scale, a.sign, (-a).sign, BigDecimal("0").sign, BigDecimal("-0").sign, BigDecimal("NaN").sign, BigDecimal("Infinity").sign, BigDecimal("-Infinity").sign, a.exponent, BigDecimal("100").exponent, BigDecimal("0.001").exponent, BigDecimal("0").exponent, a.zero?, BigDecimal("0").zero?, a.nonzero?, BigDecimal("0").nonzero?, a.finite?, BigDecimal("Infinity").finite?, BigDecimal("Infinity").infinite?, BigDecimal("-Infinity").infinite?, a.infinite?, BigDecimal("NaN").nan?, a.nan?, a.positive?, a.negative?, (-a).negative?, a.integer?, BigDecimal("3").integer?, a.split, BigDecimal("-100").split, BigDecimal("0.001").split, BigDecimal("0").split, a.coerce(2), a.coerce(2.5), a.coerce(BigDecimal("1")), BigDecimal("1").frozen?, a.sqrt(4), a._dump, BigDecimal._load("18:0.123e1"), a.sign.class, a.truncate(1).class] }
t("consts") { [BigDecimal::VERSION, BigDecimal::ROUND_MODE, BigDecimal::ROUND_UP, BigDecimal::ROUND_DOWN, BigDecimal::ROUND_HALF_UP, BigDecimal::ROUND_HALF_DOWN, BigDecimal::ROUND_CEILING, BigDecimal::ROUND_FLOOR, BigDecimal::ROUND_HALF_EVEN, BigDecimal::SIGN_NaN, BigDecimal::SIGN_POSITIVE_ZERO, BigDecimal::SIGN_NEGATIVE_ZERO, BigDecimal::SIGN_POSITIVE_FINITE, BigDecimal::SIGN_NEGATIVE_FINITE, BigDecimal::SIGN_POSITIVE_INFINITE, BigDecimal::SIGN_NEGATIVE_INFINITE, BigDecimal::EXCEPTION_ALL, BigDecimal::EXCEPTION_NaN, BigDecimal::EXCEPTION_INFINITY, BigDecimal::EXCEPTION_UNDERFLOW, BigDecimal::EXCEPTION_OVERFLOW, BigDecimal::EXCEPTION_ZERODIVIDE, BigDecimal::BASE, BigDecimal::INFINITY, BigDecimal::NAN, BigDecimal.mode(BigDecimal::ROUND_MODE), BigDecimal.mode(BigDecimal::EXCEPTION_ALL), BigDecimal.limit, BigDecimal.double_fig, BigDecimal.superclass, BigDecimal.ancestors.include?(Comparable)] }
t("mode") { old = BigDecimal.mode(BigDecimal::ROUND_MODE); BigDecimal.mode(BigDecimal::ROUND_MODE, :half_even); r = [BigDecimal("2.5").round, BigDecimal("3.5").round, BigDecimal.mode(BigDecimal::ROUND_MODE)]; BigDecimal.mode(BigDecimal::ROUND_MODE, old); r + [BigDecimal("2.5").round] }
t("limit") { old = BigDecimal.limit; BigDecimal.limit(3); r = [BigDecimal("1.23456") + 0, BigDecimal("1.23456") * 1, BigDecimal("1") / 3, BigDecimal("1.23456"), BigDecimal.limit]; BigDecimal.limit(old); r }
t("save") { BigDecimal.save_rounding_mode { BigDecimal.mode(BigDecimal::ROUND_MODE, :down); BigDecimal("2.7").round } }
t("save2") { [BigDecimal.save_rounding_mode { 1 }, BigDecimal.mode(BigDecimal::ROUND_MODE)] }
t("exc2") { BigDecimal("NaN") + 1 }
t("exc3") { BigDecimal("Infinity") - BigDecimal("Infinity") }
t("exc4") { BigDecimal("Infinity") + 1 }
t("exc5") { BigDecimal("Infinity") * 0 }
t("exc6") { BigDecimal.mode(BigDecimal::EXCEPTION_INFINITY, true); r = begin; BigDecimal("1") / 0; rescue => e; e; end; BigDecimal.mode(BigDecimal::EXCEPTION_INFINITY, false); r }
t("exc7") { BigDecimal.mode(BigDecimal::EXCEPTION_ZERODIVIDE, true); r = begin; BigDecimal("1") / 0; rescue => e; e; end; BigDecimal.mode(BigDecimal::EXCEPTION_ZERODIVIDE, false); r }
t("exc8") { BigDecimal.mode(BigDecimal::EXCEPTION_NaN, true); r = begin; BigDecimal("NaN") + 1; rescue => e; e; end; BigDecimal.mode(BigDecimal::EXCEPTION_NaN, false); r }
t("math") { [BigMath.PI(10), BigMath.E(10), BigMath.sqrt(BigDecimal("2"), 10), BigMath.sin(BigDecimal("1"), 10), BigMath.cos(BigDecimal("1"), 10), BigMath.atan(BigDecimal("1"), 10), BigMath.log(BigDecimal("2"), 10), BigMath.exp(BigDecimal("1"), 10), BigMath.log(BigDecimal("10"), 20)] }
t("math2") { BigMath.log(BigDecimal("-1"), 10) }
t("power") { [BigDecimal("2") ** 10, BigDecimal("2") ** -2, BigDecimal("1.1") ** 2, BigDecimal("2") ** 0, BigDecimal("0") ** 0, BigDecimal("2") ** BigDecimal("3"), BigDecimal("4") ** 0.5, BigDecimal("2") ** 0.5, BigDecimal("2") ** 100, BigDecimal("2").power(3), BigDecimal("2").power(3, 5), BigDecimal("3").power(-1, 10), BigDecimal("1.5") ** 2.5, BigDecimal("10") ** -3, BigDecimal("-2") ** 3] }
t("power2") { BigDecimal("-2") ** 0.5 }
t("power3") { BigDecimal("0") ** -1 }
t("int_ops") { [1 + a, 1 - a, 2 * a, 3 / a, 1.5 + a, 1.5 * a, 2 ** BigDecimal("2"), 10 % a, 1 <=> a, 1 == a, 2 > a, [1, a, 1.5].max, [1, a, 1.5].sort, 1.coerce(a), 1.5.coerce(a), a.coerce(1)] }
t("hash") { [{a => 1}[BigDecimal("1.23")], {a => 1}[BigDecimal("1.230")], {a => 1}[1.23], [a, BigDecimal("1.230")].uniq.size, BigDecimal("1.0").hash == BigDecimal("1").hash, BigDecimal("1.0").eql?(BigDecimal("1")), BigDecimal("1") == 1, BigDecimal("1").eql?(1), BigDecimal("1").hash == 1.hash] }
t("misc") { [a.frozen?, a.dup, a.clone, a.class, a.is_a?(Numeric), a.kind_of?(Comparable), a.respond_to?(:to_d), a.to_s.frozen?, a.nonzero?, BigDecimal("0").nonzero?, a.divmod(1), a.abs2, a.ceil(10), BigDecimal("1.5").floor(-1), BigDecimal("15").round(-1), BigDecimal("1.23456789").round(4), (a..b).include?(2), a.integer?, a.zero?, a.div(1), a.quo(1)] }
t("misc2") { [BigDecimal("1.23456789").round(4).to_s, BigDecimal("9.995").round(2).to_s, BigDecimal("9.994").round(2).to_s, BigDecimal("-9.995").round(2).to_s, BigDecimal("0.5").round.to_s, BigDecimal("1.5").round.to_s, BigDecimal("-0.5").round.to_s, BigDecimal("0.5").round(0, :half_even).to_s, BigDecimal("1.5").round(0, :half_even).to_s, BigDecimal("0.05").round(1).to_s, BigDecimal("0.15").round(1).to_s, BigDecimal("0.25").round(1).to_s, BigDecimal("2.675").round(2).to_s] }
t("mult_prec") { [a.mult(b, 0), a.mult(b, 1), a.mult(b, 2), a.add(b, 0), a.add(b, 1), a.sub(b, 1), a.div(b, 0), a.div(b, 1), BigDecimal("1").div(BigDecimal("3"), 1), BigDecimal("1").div(BigDecimal("3"), 2), BigDecimal("2").div(BigDecimal("3"), 2), BigDecimal("2").div(BigDecimal("3"), 1), BigDecimal("12.34").add(BigDecimal("0.0056"), 3), BigDecimal("12.34").mult(BigDecimal("0.0056"), 3)] }

__END__
new: [0.123e1, 0.45e1, 0.1e1, -0.1e-2, 0.1e11, 0.15e-2, 0.0, -0.0, 0.15e1, 0.10005e4, 0.3e1, -0.7e1, 0.1180591620717411303424e22, 0.15e1, 0.3333333333e0, 0.123e1, Infinity, -Infinity, NaN, 0.15e1, 0.5e0, 0.1e4, 0.0, 0.123456789012345678901234567890123456789e30]
new_err: ArgumentError: invalid value for BigDecimal(): "abc"
new_err2: ArgumentError: invalid value for BigDecimal(): "1.2.3"
new_err3: ArgumentError: can't omit precision for a Float.
new_err4: TypeError: can't convert nil into BigDecimal
new_err5: ArgumentError: invalid value for BigDecimal(): ""
new_err6: ArgumentError: invalid value for BigDecimal(): "1 2"
new_err7: ArgumentError: invalid value for BigDecimal(): "0x10"
new_err8: ArgumentError: invalid value for BigDecimal(): "1e"
new_exc: [nil, nil]
to_s: ["0.123e1", "0.45e1", "0.1e3", "0.1e-2", "-0.15e1", "0.0", "-0.0", "0.1e21", "0.15e-6", "Infinity", "NaN", "-Infinity", "123.456", "+123.456", " 123.456", "100.0", "0.001", "-0.5", "0.0", "100000000000000000000.0", "1 234 567.891", "0.123 456 789 1e7", "0.123 456 789 1e7", "0.1234567891e7", "+0.1234567891e7", " 0.1234567891e7"]
inspect: ["0.123e1", "0.1e3", "0.1e-2", "0.1e21", "-0.0"]
arith: [0.573e1, -0.327e1, 0.5535e1, 0.273333333333333333333333333333333333e0, 0.3658536585365853658536585365853658537e1, 0.123e1, 0.81e0, -0.123e1, 0.123e1, 0.15129e1, 0.8130081300813008130081300813008130081300813e0, 0.2121320343559642573202533086314547e1, 0.123e1, 0.123e1, 0.223e1, -0.27e0, 0.246e1, 0.615e0, 0.223e1, 0.246e1, 0.27e0, 0.8130081300813008130081300813008130082e1, 0.57e1, -0.33e1, 0.554e1, 0.27333e0, 0, 3, 0.273333333333333333333333333333333333e0, [0.0, 0.123e1], [0.3e1, 0.81e0], 0.123e1, 0.81e0, 0.273333333333333333333333333333333333e0]
arith2: [0.333333333333333333333333333333333333e0, 0.666666666666666666666666666666666667e0, 0.142857142857142857142857142857142857e0, 0.25e1, 0.333333333333333333333333333333333333e0, 0.3e0, 0.3e0, 0.100000000000000000001e1, 0.12193263113702179522374638011112635269e40, 0.999999999999999999999999999999999999e0, 0.1414213562373095e1, 0.14142135623730950488016887242e1, 0.4e1, 0.0, Infinity, -Infinity, NaN]
arith3: Infinity
arith4: ZeroDivisionError: divided by 0
arith5: FloatDomainError: sqrt of negative value
arith6: [0.333333333333333333333333333333333333e0, 36, 36, 36, 0.33333333333333333333333333333333333333333333333333e0, 0.333333333333333333333333333333333333e0, 0.314e1]
cmp: [true, false, true, false, true, true, true, true, -1, 1, 0, 1, nil, false, true, [0.123e1, 0.45e1], 0.45e1, true, 2, true, true, true, true, false, true, true, true]
cmp2: ArgumentError: comparison of BigDecimal with String failed
round: [1, 0.12e1, 0.123e1, 5, 3, -3, 4, 0.2e1, 0.4e1, 0.2e1, 0.2e1, 0.3e1, 0.3e1, -0.3e1, 0.3e1, 0.2e1, 0.1235e1, 0.1234e1, 0, 1200, Integer, BigDecimal, 1, 2, 1, 0.1e1, 0.23e0, -2, -1, -1, -0.23e0, 0.12e1, 0.13e1, 0.12e1, 0.19e1, 0.2e1, Integer, BigDecimal, 4, 1, -1, 1, Integer]
round2: ArgumentError: invalid rounding mode (bogus)
round3: TypeError: no implicit conversion of String into Integer
conv: [1.23, 1, "0.123e1", 0.123e1, 4.5, 1.0e+30, 0.0, Infinity, Infinity, true]
conv3: FloatDomainError: Computation results in 'Infinity'
conv2: FloatDomainError: Computation results in 'NaN' (Not a Number)
util: [0.15e1, 0.0, 0.0, 0.3e1, 0.15e1, 0.1e0, 0.3333333333333333e0, 0.15e1, 0.1e0, 0.0, 0.123e1, 0.1e4, 0.25e1, 0.1e21, 0.1180591620717411303424e22, BigDecimal, BigDecimal, 0.3e0]
util2: ["1.23", "100.0", "0.001", "-1.5", "100000000000000000000.0", "1.0"]
props: [3, 2, [3, 2], 3, 3, 0, 1, 3, 3, 0, 0, 6, 3, 21, 20, 2, -2, 1, -1, 0, 3, -3, 1, 3, -2, 0, false, true, 0.123e1, nil, true, false, 1, -1, nil, true, false, true, false, true, false, false, [1, "123", 10, 1], [-1, "1", 10, 3], [1, "1", 10, -2], [1, "0", 10, 0], [0.2e1, 0.123e1], [0.25e1, 0.123e1], [0.1e1, 0.123e1], true, 0.1109053650640941e1, "18:0.123e1", 0.123e1, Integer, BigDecimal]
consts: ["3.1.8", 256, 1, 2, 3, 4, 5, 6, 7, 0, 1, -1, 2, -2, 3, -3, 255, 2, 1, 4, 1, 16, 1000000000, Infinity, NaN, 3, 0, 0, 16, Numeric, true]
mode: [2, 4, 7, 3]
limit: [0.123e1, 0.123e1, 0.333e0, 0.123456e1, 3]
save: 2
save2: [1, 3]
exc2: NaN
exc3: NaN
exc4: Infinity
exc5: NaN
exc6: #<FloatDomainError: Computation results in 'Infinity'>
exc7: #<FloatDomainError: Divide by zero>
exc8: #<FloatDomainError: Computation results in 'NaN' (Not a Number)>
math: [0.3141592654e1, 0.2718281828e1, 0.1414213562e1, 0.8414709848e0, 0.5403023059e0, 0.7853981634e0, 0.6931471806e0, 0.2718281828e1, 0.2302585092994045684e1]
math2: Math::DomainError: Zero or negative argument for log
power: [0.1024e4, 0.25e0, 0.121e1, 0.1e1, 0.1e1, 0.8e1, 0.2e1, 0.1414213562373095048801689e1, 0.1267650600228229401496703205376e31, 0.8e1, 0.8e1, 0.3333333333e0, 0.2755675960631075360471944584044128e1, 0.1e-2, -0.8e1]
power2: Math::DomainError: Zero or negative argument for log
power3: Infinity
int_ops: [0.223e1, -0.23e0, 0.246e1, 0.2439024390243902439024390243902439025e1, 0.273e1, 0.1845e1, 0.4e1, 0.16e0, -1, false, true, 1.5, [1, 0.123e1, 1.5], [1.23, 1.0], [1.23, 1.5], [0.1e1, 0.123e1]]
hash: [1, 1, nil, 1, true, true, true, true, false]
misc: [true, 0.123e1, 0.123e1, BigDecimal, true, true, true, true, 0.123e1, nil, [0.1e1, 0.23e0], 0.15129e1, 0.123e1, 0.0, 20, 0.12346e1, true, false, false, 1, 0.123e1]
misc2: ["0.12346e1", "0.1e2", "0.999e1", "-0.1e2", "1", "2", "-1", "0.0", "0.2e1", "0.1e0", "0.2e0", "0.3e0", "0.268e1"]
mult_prec: [0.5535e1, 0.6e1, 0.55e1, 0.573e1, 0.6e1, -0.3e1, 0.273333333333333333333333333333333333e0, 0.3e0, 0.3e0, 0.33e0, 0.67e0, 0.7e0, 0.123e2, 0.691e-1]
