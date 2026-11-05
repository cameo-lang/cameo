package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby 3.3's, with bigdecimal 3.1.5, but where
// the probe says otherwise: sqrt and BigMath give the digits asked for
// (arith2, props, math), BigDecimal::VERSION is 3.1.8, Ruby 3.4's, and a
// String is frozen (misc).
func TestBigDecimalMatchesRuby(t *testing.T) {
	out := runScript(t, `require "bigdecimal"
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
`)
	assert.Equals(t, out, `new: [0.123e1, 0.45e1, 0.1e1, -0.1e-2, 0.1e11, 0.15e-2, 0.0, -0.0, 0.15e1, 0.10005e4, 0.3e1, -0.7e1, 0.1180591620717411303424e22, 0.15e1, 0.3333333333e0, 0.123e1, Infinity, -Infinity, NaN, 0.15e1, 0.5e0, 0.1e4, 0.0, 0.123456789012345678901234567890123456789e30]
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
`)
}

// The quotients have the digits bigdecimal computes, its last digit
// rounded from its own estimate of the next; sqrt differs as above.
func TestBigDecimalDividesAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "bigdecimal"
require "bigdecimal/util"
pairs = [["1","3"],["10","3"],["100","3"],["1e9","3"],["1e10","3"],["0.1","3"],["0.001","3"],["1","0.3"],["1","300"],["1","3e9"],["123456789","3"],["1234567890","3"],["12345678901234567890","3"],["1","123456789"],["1","1234567890"],["1","12345678901234567890"],["4.5","1.23"],["10","1.23"],["2","7"],["20","7"],["200","7"],["0.2","7"],["123456789.123456789","7"],["1","7e-9"],["1e-9","7"],["1","1180591620717411303424"],["1180591620717411303424","3"],["1.23","4.5"],["1","1.23"],["100","1.23"],["1000000000","1.23"],["0.000000001","1.23"],["1.5","0.000000007"],["123456789012345678901234567890123456789","7"],["1","123456789012345678901234567890123456789"],["99999999999999999999","99999999999999999998"],["1.0000000001","3"]]
pairs.each do |a, b|
  q = BigDecimal(a) / BigDecimal(b)
  puts "#{a} / #{b} = #{q} digits=#{q.n_significant_digits} exp=#{q.exponent}"
end
puts "--- div(n)"
[[ "1","3",1],["1","3",2],["1","3",9],["1","3",10],["2","3",1],["4.5","1.23",3],["4.5","1.23",10],["1","3",0],["22","7",3]].each { |a,b,n| puts "#{a}.div(#{b}, #{n}) = #{BigDecimal(a).div(BigDecimal(b), n)}" }
puts "--- quo/fdiv"
puts BigDecimal("1").quo(BigDecimal("3")), BigDecimal("1").quo(3), BigDecimal("1").fdiv(3)
puts "--- power neg"
[["1.23",-1],["3",-1],["7",-2],["1.5",-3],["2",-10]].each { |a,n| q = BigDecimal(a) ** n; puts "#{a} ** #{n} = #{q} digits=#{q.n_significant_digits}" }
puts "--- float pow"
[["2",0.5],["4",0.5],["1.5",2.5],["2",1.5],["10",0.1]].each { |a,n| q = BigDecimal(a) ** n; puts "#{a} ** #{n} = #{q} digits=#{q.n_significant_digits}" }
puts "--- sqrt"
[["2",1],["2",5],["2",10],["2",20],["3",10],["10",10],["0.5",10],["16",10],["1e20",10]].each { |a,n| q = BigDecimal(a).sqrt(n); puts "#{a}.sqrt(#{n}) = #{q} digits=#{q.n_significant_digits}" }
puts "--- mult/add prec"
[["1.23","4.5",0],["1.23","4.5",1],["1.23","4.5",2],["1.23","4.5",3],["1.23","4.5",4],["1.23","4.5",5]].each { |a,b,n| puts "#{a}.mult(#{b}, #{n}) = #{BigDecimal(a).mult(BigDecimal(b), n)}  add=#{BigDecimal(a).add(BigDecimal(b), n)}  sub=#{BigDecimal(a).sub(BigDecimal(b), n)}" }
puts "--- float to bigdecimal"
[[1.5, 0], [0.1, 0], [1.0/3, 0], [1.0/3, 5], [123.456, 0], [1e20, 0], [1.1, 20], [0.1, 15], [0.1, 16], [0.1, 17], [0.1, 18]].each { |f, n| begin; puts "BigDecimal(#{f}, #{n}) = #{BigDecimal(f, n)}"; rescue => e; puts "BigDecimal(#{f}, #{n}) = #{e.class}: #{e.message}"; end }
[1.5, 0.1, 1.0/3, 123.456, 1e20, 1e-5, 2.5e-7, 1e100, 1.7976931348623157e308].each { |f| puts "#{f}.to_d = #{f.to_d}  to_d(3)=#{f.to_d(3)}" }
puts "--- to_s F forms"
["0.000123", "123000", "1.5e-10", "1e-1", "0.1e1", "-123.456e5", "12345678901234567890.123456789"].each { |s| d = BigDecimal(s); puts "#{s}: to_s=#{d} F=#{d.to_s("F")} 3F=#{d.to_s("3F")} digits=#{d.to_digits} exp=#{d.exponent} prec=#{d.precision} scale=#{d.scale} split=#{d.split.inspect}" }
puts "--- parsing"
["1.", "+.5", "-.5", "1e+2", "1E-2", "1_0.5_5", "  12  ", "12abc", "1 e2", "1.5e", "e5", "Infinity ", "-Infinity", "+Infinity", "nan", "inf", "1e1000000000", "1e-1000000000", "0.0e5", "00012.5000", "1__0"].each { |s| begin; puts "#{s.inspect} => #{BigDecimal(s)}"; rescue => e; puts "#{s.inspect} => #{e.class}: #{e.message}"; end }
puts "--- integer ops"
puts (BigDecimal("7") % 2), (BigDecimal("-7") % 2), (BigDecimal("7") % -2), BigDecimal("7").remainder(-2), BigDecimal("-7").remainder(2), BigDecimal("7.5").divmod(2).inspect, BigDecimal("-7.5").divmod(2).inspect, BigDecimal("7.5").div(2), BigDecimal("-7.5").div(2), BigDecimal("7.5").modulo(BigDecimal("2.5")), BigDecimal("1").div(BigDecimal("0.3"))
puts "--- limit"
BigDecimal.limit(5)
puts BigDecimal("1.23456789"), BigDecimal("1.23456789") + 0, BigDecimal("1") / 3, BigDecimal("123456.789") * 1, BigDecimal("1.23456789").add(0, 8), BigDecimal("12345678") + 1, BigDecimal("1.23456") * BigDecimal("1.23456")
BigDecimal.limit(0)
puts "--- rounding of exact ops"
puts BigDecimal("0.1") * BigDecimal("0.2"), BigDecimal("1e-5") * BigDecimal("1e-5"), BigDecimal("123456789.123456789") * BigDecimal("987654321.987654321"), BigDecimal("1e30") + BigDecimal("1e-30")
puts "--- round with digits beyond"
puts BigDecimal("1.23456").round(10), BigDecimal("1.23456").floor(10), BigDecimal("12345").round(-10), BigDecimal("12345").floor(-2), BigDecimal("12345").ceil(-2), BigDecimal("-12345").truncate(-2), BigDecimal("0").round(2), BigDecimal("1.5").round(0, :banker)
`)
	assert.Equals(t, out, `1 / 3 = 0.333333333333333333333333333333333333e0 digits=36 exp=0
10 / 3 = 0.3333333333333333333333333333333333333e1 digits=37 exp=1
100 / 3 = 0.33333333333333333333333333333333333333e2 digits=38 exp=2
1e9 / 3 = 0.333333333333333333333333333333333333e9 digits=36 exp=9
1e10 / 3 = 0.3333333333333333333333333333333333333e10 digits=37 exp=10
0.1 / 3 = 0.33333333333333333333333333333333333333333333e-1 digits=44 exp=-1
0.001 / 3 = 0.333333333333333333333333333333333333333333e-3 digits=42 exp=-3
1 / 0.3 = 0.3333333333333333333333333333e1 digits=28 exp=1
1 / 300 = 0.3333333333333333333333333333333333e-2 digits=34 exp=-2
1 / 3e9 = 0.333333333333333333333333333333333333e-9 digits=36 exp=-9
123456789 / 3 = 0.41152263e8 digits=8 exp=8
1234567890 / 3 = 0.41152263e9 digits=8 exp=9
12345678901234567890 / 3 = 0.411522630041152263e19 digits=18 exp=19
1 / 123456789 = 0.8100000073710000670761006104e-8 digits=28 exp=-8
1 / 1234567890 = 0.810000007371000067076100610392515555e-9 digits=36 exp=-9
1 / 12345678901234567890 = 0.810000007290000066347100603757805494196111e-19 digits=42 exp=-19
4.5 / 1.23 = 0.3658536585365853658536585365853658537e1 digits=37 exp=1
10 / 1.23 = 0.8130081300813008130081300813008130082e1 digits=37 exp=1
2 / 7 = 0.285714285714285714285714285714285714e0 digits=36 exp=0
20 / 7 = 0.2857142857142857142857142857142857143e1 digits=37 exp=1
200 / 7 = 0.28571428571428571428571428571428571429e2 digits=38 exp=2
0.2 / 7 = 0.28571428571428571428571428571428571428571429e-1 digits=44 exp=-1
123456789.123456789 / 7 = 0.17636684160493827e8 digits=17 exp=8
1 / 7e-9 = 0.142857142857142857142857142857142857e9 digits=36 exp=9
1e-9 / 7 = 0.142857142857142857142857142857142857e-9 digits=36 exp=-9
1 / 1180591620717411303424 = 0.847032947254300339068322500679641962051392e-21 digits=42 exp=-21
1180591620717411303424 / 3 = 0.393530540239137101141333333333333333333333333333e21 digits=48 exp=21
1.23 / 4.5 = 0.273333333333333333333333333333333333e0 digits=36 exp=0
1 / 1.23 = 0.813008130081300813008130081300813009e0 digits=36 exp=0
100 / 1.23 = 0.81300813008130081300813008130081300814e2 digits=38 exp=2
1000000000 / 1.23 = 0.813008130081300813008130081300813009e9 digits=36 exp=9
0.000000001 / 1.23 = 0.813008130081300813008130081300813009e-9 digits=36 exp=-9
1.5 / 0.000000007 = 0.214285714285714285714285714285714286e9 digits=36 exp=9
123456789012345678901234567890123456789 / 7 = 0.17636684144620811271604938270017636684142857142857142857142857142857142857142857143e38 digits=83 exp=38
1 / 123456789012345678901234567890123456789 = 0.8100000072900000663390006036849054935327209911477529194445515669454192592033153e-38 digits=79 exp=-38
99999999999999999999 / 99999999999999999998 = 0.1000000000000000000010000000000000000000200001e1 digits=46 exp=1
1.0000000001 / 3 = 0.333333333366666666666666666666666667e0 digits=36 exp=0
--- div(n)
1.div(3, 1) = 0.3e0
1.div(3, 2) = 0.33e0
1.div(3, 9) = 0.333333333e0
1.div(3, 10) = 0.3333333333e0
2.div(3, 1) = 0.7e0
4.5.div(1.23, 3) = 0.366e1
4.5.div(1.23, 10) = 0.3658536585e1
1.div(3, 0) = 0.333333333333333333333333333333333333e0
22.div(7, 3) = 0.314e1
--- quo/fdiv
0.333333333333333333333333333333333333e0
0.333333333333333333333333333333333333e0
0.3333333333333333
--- power neg
1.23 ** -1 = 0.8130081300813008130081300813008130081300813e0 digits=43
3 ** -1 = 0.333333333333333333333333333e0 digits=27
7 ** -2 = 0.20408163265306122448979591836734693e-1 digits=35
1.5 ** -3 = 0.296296296296296296296296296296296296296296296296296296296296296296296296296296296e0 digits=81
2 ** -10 = 0.9765625e-3 digits=7
--- float pow
2 ** 0.5 = 0.1414213562373095048801689e1 digits=25
4 ** 0.5 = 0.2e1 digits=1
1.5 ** 2.5 = 0.2755675960631075360471944584044128e1 digits=34
2 ** 1.5 = 0.2828427124746190097603377e1 digits=25
10 ** 0.1 = 0.1258925411794167210423954e1 digits=25
--- sqrt
2.sqrt(1) = 0.1414213562373095e1 digits=16
2.sqrt(5) = 0.1414213562373095e1 digits=16
2.sqrt(10) = 0.1414213562373095e1 digits=16
2.sqrt(20) = 0.14142135623730950488e1 digits=20
3.sqrt(10) = 0.1732050807568877e1 digits=16
10.sqrt(10) = 0.3162277660168379e1 digits=16
0.5.sqrt(10) = 0.7071067811865475e0 digits=16
16.sqrt(10) = 0.4e1 digits=1
1e20.sqrt(10) = 0.1e11 digits=1
--- mult/add prec
1.23.mult(4.5, 0) = 0.5535e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 1) = 0.6e1  add=0.6e1  sub=-0.3e1
1.23.mult(4.5, 2) = 0.55e1  add=0.57e1  sub=-0.33e1
1.23.mult(4.5, 3) = 0.554e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 4) = 0.5535e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 5) = 0.5535e1  add=0.573e1  sub=-0.327e1
--- float to bigdecimal
BigDecimal(1.5, 0) = 0.15e1
BigDecimal(0.1, 0) = 0.1e0
BigDecimal(0.3333333333333333, 0) = 0.3333333333333333e0
BigDecimal(0.3333333333333333, 5) = 0.33333e0
BigDecimal(123.456, 0) = 0.123456e3
BigDecimal(1.0e+20, 0) = 0.1e21
BigDecimal(1.1, 20) = ArgumentError: precision too large.
BigDecimal(0.1, 15) = 0.1e0
BigDecimal(0.1, 16) = 0.1e0
BigDecimal(0.1, 17) = ArgumentError: precision too large.
BigDecimal(0.1, 18) = ArgumentError: precision too large.
1.5.to_d = 0.15e1  to_d(3)=0.15e1
0.1.to_d = 0.1e0  to_d(3)=0.1e0
0.3333333333333333.to_d = 0.3333333333333333e0  to_d(3)=0.333e0
123.456.to_d = 0.123456e3  to_d(3)=0.123e3
1.0e+20.to_d = 0.1e21  to_d(3)=0.1e21
1.0e-05.to_d = 0.1e-4  to_d(3)=0.1e-4
2.5e-07.to_d = 0.25e-6  to_d(3)=0.25e-6
1.0e+100.to_d = 0.1e101  to_d(3)=0.1e101
1.7976931348623157e+308.to_d = 0.1797693134862315e309  to_d(3)=0.18e309
--- to_s F forms
0.000123: to_s=0.123e-3 F=0.000123 3F=0.000 123 digits=0.000123 exp=-3 prec=6 scale=6 split=[1, "123", 10, -3]
123000: to_s=0.123e6 F=123000.0 3F=123 000.0 digits=123000.0 exp=6 prec=6 scale=0 split=[1, "123", 10, 6]
1.5e-10: to_s=0.15e-9 F=0.00000000015 3F=0.000 000 000 15 digits=0.00000000015 exp=-9 prec=11 scale=11 split=[1, "15", 10, -9]
1e-1: to_s=0.1e0 F=0.1 3F=0.1 digits=0.1 exp=0 prec=1 scale=1 split=[1, "1", 10, 0]
0.1e1: to_s=0.1e1 F=1.0 3F=1.0 digits=1.0 exp=1 prec=1 scale=0 split=[1, "1", 10, 1]
-123.456e5: to_s=-0.123456e8 F=-12345600.0 3F=-12 345 600.0 digits=-12345600.0 exp=8 prec=8 scale=0 split=[-1, "123456", 10, 8]
12345678901234567890.123456789: to_s=0.12345678901234567890123456789e20 F=12345678901234567890.123456789 3F=12 345 678 901 234 567 890.123 456 789 digits=12345678901234567890.123456789 exp=20 prec=29 scale=9 split=[1, "12345678901234567890123456789", 10, 20]
--- parsing
"1." => ArgumentError: invalid value for BigDecimal(): "1."
"+.5" => 0.5e0
"-.5" => -0.5e0
"1e+2" => 0.1e3
"1E-2" => 0.1e-1
"1_0.5_5" => 0.1055e2
"  12  " => 0.12e2
"12abc" => ArgumentError: invalid value for BigDecimal(): "12abc"
"1 e2" => ArgumentError: invalid value for BigDecimal(): "1 e2"
"1.5e" => ArgumentError: invalid value for BigDecimal(): "1.5e"
"e5" => ArgumentError: invalid value for BigDecimal(): "e5"
"Infinity " => Infinity
"-Infinity" => -Infinity
"+Infinity" => Infinity
"nan" => ArgumentError: invalid value for BigDecimal(): "nan"
"inf" => ArgumentError: invalid value for BigDecimal(): "inf"
"1e1000000000" => 0.1e1000000001
"1e-1000000000" => 0.1e-999999999
"0.0e5" => 0.0
"00012.5000" => 0.125e2
"1__0" => ArgumentError: invalid value for BigDecimal(): "1__0"
--- integer ops
0.1e1
0.1e1
-0.1e1
0.1e1
-0.1e1
[0.3e1, 0.15e1]
[-0.4e1, 0.5e0]
3
-4
0.0
3
--- limit
0.123456789e1
0.12346e1
0.33333e0
0.12346e6
0.12345679e1
0.12346e8
0.15241e1
--- rounding of exact ops
0.2e-1
0.1e-9
0.121932631356500531347203169112635269e18
0.1000000000000000000000000000000000000000000000000000000000001e31
--- round with digits beyond
0.123456e1
0.123456e1
0
0.123e5
0.124e5
-0.123e5
0.0
0.2e1
`)
}

func TestBigDecimalIsAValueAndCoerces(t *testing.T) {
	out := runScript(t, `require "bigdecimal"
a = BigDecimal("1.23")
p Set[a, BigDecimal("1.230"), BigDecimal("1.24")].size, {a => :x}[BigDecimal("1.230")], a.frozen?
p 1 + a, 2.5 * a, 10 / a, 1 == a, 1.23 == a, 1 < a, [a, 1, 1.5].sort, [a, 1, 1.5].max, 1.clamp(a, 2), a.clamp(1, 2), 2.between?(a, 3)
p Integer === a, Numeric === a, a.is_a?(Comparable), a.class.ancestors.take(3)
`)
	assert.Equals(t, out, "2\n:x\ntrue\n0.223e1\n0.3075e1\n0.8130081300813008130081300813008130082e1\nfalse\ntrue\ntrue\n[1, 0.123e1, 1.5]\n1.5\n0.123e1\n0.123e1\ntrue\nfalse\ntrue\ntrue\n[BigDecimal, Numeric, Comparable]\n")
}

// A number compares with a value that takes part in coerce, a BigDecimal,
// and with nothing else: Ruby's Integer#<=> answers nil for an object with
// a <=> of its own but no coerce.
func TestNumbersCompareOnlyWithCoercibleValues(t *testing.T) {
	out := runScript(t, `require "bigdecimal"
class V
  def <=>(o); 1; end
end
p(1 <=> V.new, 1.5 <=> V.new, 1 <=> BigDecimal("2"), 3.0 <=> BigDecimal("2"), 1 == V.new)
`)
	assert.Equals(t, out, `nil
nil
-1
1
false
`)
}
