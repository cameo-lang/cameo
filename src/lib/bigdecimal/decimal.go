package bigdecimal

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// dec is a decimal number: sign × coef × 10^exp, with coef a non-negative
// integer without trailing zeros, or one of the specials. Ruby shows a
// BigDecimal as 0.digits × 10^exponent; that exponent is exp plus the
// digit count of coef.
type dec struct {
	special special
	neg     bool
	coef    *big.Int
	exp     int64
}

type special int8

const (
	finite special = iota
	nan
	posInf
	negInf
)

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
	bigTen  = big.NewInt(10)
)

func zero(neg bool) dec { return dec{neg: neg, coef: new(big.Int)} }
func decNaN() dec       { return dec{special: nan, coef: new(big.Int)} }
func decInf(neg bool) dec {
	return dec{special: posInf + special(b2i(neg)), neg: neg, coef: new(big.Int)}
}

func b2i(b bool) int8 {
	if b {
		return 1
	}
	return 0
}

func (d dec) isNaN() bool  { return d.special == nan }
func (d dec) isInf() bool  { return d.special == posInf || d.special == negInf }
func (d dec) finite() bool { return d.special == finite }
func (d dec) isZero() bool { return d.finite() && d.coef.Sign() == 0 }
func (d dec) sign() int    { return map[bool]int{true: -1, false: 1}[d.neg] }
func (d dec) ndigits() int { return len(d.coef.String()) }
func (d dec) exponent() int64 { // Ruby's exponent: 0.123e1 has 1
	if d.isZero() || !d.finite() {
		return 0
	}
	return d.exp + int64(d.ndigits())
}

// norm strips trailing zeros from the coefficient.
func norm(neg bool, coef *big.Int, exp int64) dec {
	if coef.Sign() == 0 {
		return zero(neg)
	}
	c := new(big.Int).Set(coef)
	q, r := new(big.Int), new(big.Int)
	for {
		q.QuoRem(c, bigTen, r)
		if r.Sign() != 0 {
			break
		}
		c.Set(q)
		exp++
	}
	return dec{neg: neg, coef: c, exp: exp}
}

func fromInt(n *big.Int) dec {
	return norm(n.Sign() < 0, new(big.Int).Abs(n), 0)
}

// pow10 is 10^n for n >= 0.
func pow10(n int64) *big.Int {
	return new(big.Int).Exp(bigTen, big.NewInt(n), nil)
}

// digits are the coefficient's decimal digits.
func (d dec) digits() string { return d.coef.String() }

// precision is BigDecimal#precision: the digit positions from the first
// significant one to the last, or to the units.
func (d dec) precision() int64 {
	if d.isZero() || !d.finite() {
		return 0
	}
	nd := int64(d.ndigits())
	e := d.exponent()
	if e > 0 {
		return max(e, nd)
	}
	return nd - e
}

// scale is BigDecimal#scale: the digits after the point.
func (d dec) scale() int64 {
	if d.isZero() || !d.finite() {
		return 0
	}
	return max(0, -d.exp)
}

// Parsing.

// parse reads Ruby's BigDecimal syntax: spaces around, a sign, digits with
// single underscores between them, a fraction, an exponent, or Infinity
// and NaN.
func parse(s string) (dec, bool) {
	t := strings.TrimSpace(s)
	switch t {
	case "Infinity", "+Infinity":
		return decInf(false), true
	case "-Infinity":
		return decInf(true), true
	case "NaN":
		return decNaN(), true
	}
	i := 0
	neg := false
	if i < len(t) && (t[i] == '+' || t[i] == '-') {
		neg = t[i] == '-'
		i++
	}
	digits := func() (string, bool) {
		var b strings.Builder
		start := i
		for i < len(t) {
			c := t[i]
			switch {
			case c >= '0' && c <= '9':
				b.WriteByte(c)
			case c == '_' && i > start && i+1 < len(t) && t[i+1] >= '0' && t[i+1] <= '9' && t[i-1] != '_':
			default:
				return b.String(), b.Len() > 0
			}
			i++
		}
		return b.String(), b.Len() > 0
	}
	intPart, hasInt := digits()
	fracPart := ""
	if i < len(t) && t[i] == '.' {
		i++
		var hasFrac bool
		fracPart, hasFrac = digits()
		if !hasFrac {
			return dec{}, false
		}
	}
	if !hasInt && fracPart == "" {
		return dec{}, false
	}
	var exp int64
	if i < len(t) && (t[i] == 'e' || t[i] == 'E') {
		i++
		eneg := false
		if i < len(t) && (t[i] == '+' || t[i] == '-') {
			eneg = t[i] == '-'
			i++
		}
		e, ok := digits()
		if !ok {
			return dec{}, false
		}
		n, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			return dec{}, false
		}
		if eneg {
			n = -n
		}
		exp = n
	}
	if i != len(t) {
		return dec{}, false
	}
	coef, ok := new(big.Int).SetString(intPart+fracPart, 10)
	if !ok {
		coef = new(big.Int)
	}
	return norm(neg, coef, exp-int64(len(fracPart))), true
}

// parseLoose reads the longest number at the start, as String#to_d does;
// nothing readable is zero.
func parseLoose(s string) dec {
	t := strings.TrimLeft(s, " \t\n\r\f\v")
	end := 0
	seenDigit, seenDot, seenE := false, false, false
	for end < len(t) {
		c := t[end]
		switch {
		case c >= '0' && c <= '9':
			seenDigit = true
		case (c == '+' || c == '-') && (end == 0 || t[end-1] == 'e' || t[end-1] == 'E'):
		case c == '.' && !seenDot && !seenE:
			seenDot = true
		case (c == 'e' || c == 'E') && seenDigit && !seenE:
			seenE = true
		case c == '_' && seenDigit:
		default:
			goto done
		}
		end++
	}
done:
	for end > 0 {
		if d, ok := parse(t[:end]); ok {
			return d
		}
		end--
	}
	return zero(false)
}

// fromFloat converts a Float as BigDecimal(f, prec) does: the shortest
// digits that read back as f, rounded to prec significant digits when
// prec is positive.
func fromFloat(f float64, prec int64, mode int) dec {
	switch {
	case math.IsNaN(f):
		return decNaN()
	case math.IsInf(f, 1):
		return decInf(false)
	case math.IsInf(f, -1):
		return decInf(true)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	d, _ := parse(s)
	if prec > 0 {
		d = roundSig(d, prec, mode)
	} else {
		// Without a precision, bigdecimal keeps Float::DIG + 1 digits.
		d = roundSig(d, 16, roundDown)
	}
	return d
}

// Formatting.

// engineering is to_s: 0.123e1, with a sign prefix for "+" or " ".
func (d dec) engineering(prefix string, group int) string {
	s := d.plainSpecial()
	if s != "" {
		return s
	}
	var b strings.Builder
	if d.neg {
		b.WriteString("-")
	} else {
		b.WriteString(prefix)
	}
	if d.isZero() {
		b.WriteString("0.0")
		return b.String()
	}
	b.WriteString("0.")
	b.WriteString(grouped(d.digits(), group))
	b.WriteString("e")
	b.WriteString(strconv.FormatInt(d.exponent(), 10))
	return b.String()
}

func (d dec) plainSpecial() string {
	switch d.special {
	case nan:
		return "NaN"
	case posInf:
		return "Infinity"
	case negInf:
		return "-Infinity"
	}
	return ""
}

// grouped puts a space after every group digits, from the left.
func grouped(digits string, group int) string {
	if group <= 0 || len(digits) <= group {
		return digits
	}
	var b strings.Builder
	for i := 0; i < len(digits); i += group {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(digits[i:min(i+group, len(digits))])
	}
	return b.String()
}

// plain is to_s("F"): the digits with the point in place, .0 for an
// integer; the integer part grouped from the right and the fraction from
// the left when group is positive.
func (d dec) plain(prefix string, group int) string {
	s := d.plainSpecial()
	if s != "" {
		return s
	}
	var b strings.Builder
	if d.neg {
		b.WriteString("-")
	} else {
		b.WriteString(prefix)
	}
	if d.isZero() {
		b.WriteString("0.0")
		return b.String()
	}
	digits := d.digits()
	e := d.exponent()
	var intPart, fracPart string
	switch {
	case e <= 0:
		intPart = "0"
		fracPart = strings.Repeat("0", int(-e)) + digits
	case int(e) >= len(digits):
		intPart = digits + strings.Repeat("0", int(e)-len(digits))
		fracPart = "0"
	default:
		intPart = digits[:e]
		fracPart = digits[e:]
	}
	if group > 0 {
		// The integer part is grouped from the point leftwards.
		head := len(intPart) % group
		var g strings.Builder
		if head > 0 {
			g.WriteString(intPart[:head])
		}
		for i := head; i < len(intPart); i += group {
			if g.Len() > 0 {
				g.WriteString(" ")
			}
			g.WriteString(intPart[i : i+group])
		}
		intPart = g.String()
		fracPart = grouped(fracPart, group)
	}
	b.WriteString(intPart)
	b.WriteString(".")
	b.WriteString(fracPart)
	return b.String()
}

// Rounding modes, with Ruby's numbers.

const (
	roundUp       = 1
	roundDown     = 2
	roundHalfUp   = 3
	roundHalfDown = 4
	roundCeiling  = 5
	roundFloor    = 6
	roundHalfEven = 7
)

// roundDigits drops k > 0 digits off the coefficient, rounding by mode;
// neg is the number's sign.
func roundDigits(coef *big.Int, k int64, neg bool, mode int) *big.Int {
	p := pow10(k)
	q, r := new(big.Int).QuoRem(coef, p, new(big.Int))
	if r.Sign() == 0 {
		return q
	}
	half := new(big.Int).Mul(pow10(k-1), big.NewInt(5))
	cmpHalf := r.Cmp(half)
	up := false
	switch mode {
	case roundUp:
		up = true
	case roundDown:
		up = false
	case roundHalfUp:
		up = cmpHalf >= 0
	case roundHalfDown:
		up = cmpHalf > 0
	case roundHalfEven:
		up = cmpHalf > 0 || cmpHalf == 0 && q.Bit(0) == 1
	case roundCeiling:
		up = !neg
	case roundFloor:
		up = neg
	}
	if up {
		q.Add(q, bigOne)
	}
	return q
}

// roundAt rounds to n digits after the point (n negative rounds before
// it): BigDecimal#round(n, mode), floor(n), ceil(n), truncate(n).
func roundAt(d dec, n int64, mode int) dec {
	if !d.finite() || d.isZero() {
		return d
	}
	// The coefficient's last digit is at 10^exp; drop the digits below
	// 10^-n.
	k := -n - d.exp
	if k <= 0 {
		return d
	}
	if k > int64(d.ndigits()) {
		// Everything goes; what is left is 0 or one unit at 10^-n.
		q := roundDigits(d.coef, int64(d.ndigits())+1, d.neg, mode)
		if q.Sign() == 0 {
			return zero(d.neg)
		}
		return norm(d.neg, q, -n)
	}
	return norm(d.neg, roundDigits(d.coef, k, d.neg, mode), d.exp+k)
}

// roundSig rounds to n significant digits.
func roundSig(d dec, n int64, mode int) dec {
	if !d.finite() || d.isZero() || n <= 0 {
		return d
	}
	k := int64(d.ndigits()) - n
	if k <= 0 {
		return d
	}
	return norm(d.neg, roundDigits(d.coef, k, d.neg, mode), d.exp+k)
}

// Comparison and arithmetic.

// cmp orders two finite or infinite decimals; a NaN is incomparable.
func cmp(a, b dec) (int, bool) {
	if a.isNaN() || b.isNaN() {
		return 0, false
	}
	if a.isInf() || b.isInf() {
		av, bv := infSign(a), infSign(b)
		if av == bv {
			if av == 0 {
				return 0, true
			}
			return 0, true
		}
		if av < bv {
			return -1, true
		}
		return 1, true
	}
	as, bs := a.signedCoef(), b.signedCoef()
	if a.exp == b.exp {
		return as.Cmp(bs), true
	}
	// Scale the one with the larger exponent down.
	if a.exp > b.exp {
		as.Mul(as, pow10(a.exp-b.exp))
	} else {
		bs.Mul(bs, pow10(b.exp-a.exp))
	}
	return as.Cmp(bs), true
}

func infSign(d dec) int {
	switch d.special {
	case posInf:
		return 2
	case negInf:
		return -2
	}
	return 0
}

func (d dec) signedCoef() *big.Int {
	c := new(big.Int).Set(d.coef)
	if d.neg {
		c.Neg(c)
	}
	return c
}

// aligned gives both coefficients at the smaller exponent.
func aligned(a, b dec) (as, bs *big.Int, exp int64) {
	as, bs = a.signedCoef(), b.signedCoef()
	exp = min(a.exp, b.exp)
	if a.exp > exp {
		as.Mul(as, pow10(a.exp-exp))
	}
	if b.exp > exp {
		bs.Mul(bs, pow10(b.exp-exp))
	}
	return as, bs, exp
}

func fromSigned(n *big.Int, exp int64) dec {
	return norm(n.Sign() < 0, new(big.Int).Abs(n), exp)
}

func add(a, b dec) dec {
	if a.isNaN() || b.isNaN() {
		return decNaN()
	}
	if a.isInf() || b.isInf() {
		if a.isInf() && b.isInf() && a.special != b.special {
			return decNaN()
		}
		if a.isInf() {
			return a
		}
		return b
	}
	as, bs, exp := aligned(a, b)
	sum := as.Add(as, bs)
	if sum.Sign() == 0 {
		return zero(a.neg && b.neg)
	}
	return fromSigned(sum, exp)
}

func neg(a dec) dec {
	switch a.special {
	case nan:
		return a
	case posInf:
		return decInf(true)
	case negInf:
		return decInf(false)
	}
	return dec{neg: !a.neg, coef: a.coef, exp: a.exp}
}

func sub(a, b dec) dec { return add(a, neg(b)) }

func mul(a, b dec) dec {
	if a.isNaN() || b.isNaN() {
		return decNaN()
	}
	if a.isInf() || b.isInf() {
		if a.isZero() || b.isZero() {
			return decNaN()
		}
		return decInf(a.neg != b.neg)
	}
	return norm(a.neg != b.neg && a.coef.Sign() != 0 && b.coef.Sign() != 0, new(big.Int).Mul(a.coef, b.coef), a.exp+b.exp)
}

// wordExp is the exponent in words of nine digits, as bigdecimal keeps
// a number: 0.frac × 1000000000^wordExp.
func (d dec) wordExp() int64 {
	e := d.exponent()
	if e >= 0 {
		return (e + 8) / 9
	}
	return -((-e) / 9)
}

// wordSpan is how many nine-digit words a number's digits cover, as
// bigdecimal stores it.
func (d dec) wordSpan() int64 {
	if !d.finite() || d.isZero() {
		return 1
	}
	top := floorDiv9(d.exponent()-1) + 1
	bottom := floorDiv9(d.exp) + 1
	return top - bottom + 1
}

func floorDiv9(n int64) int64 {
	if n >= 0 {
		return n / 9
	}
	return -((-n + 8) / 9)
}

// quotientWords is how many words of nine digits a / b gets: bigdecimal
// sizes the result from the operands' precision, at least 32 digits.
func quotientWords(a, b dec) int64 {
	mx := max(a.precision(), b.precision()) * 2
	mx = max(mx, 32)
	words := (mx + 18 + 8) / 9 // the result's allocation, in words
	return words - 1
}

// divide is BigDecimal#/: the quotient to the words bigdecimal computes,
// the last digit rounded by mode from the remainder.
func divide(a, b dec, mode int) dec {
	if a.isNaN() || b.isNaN() {
		return decNaN()
	}
	if a.isInf() && b.isInf() {
		return decNaN()
	}
	if a.isInf() {
		return decInf(a.neg != b.neg)
	}
	if b.isInf() {
		return zero(a.neg != b.neg)
	}
	if b.isZero() {
		if a.isZero() {
			return decNaN()
		}
		return decInf(a.neg != b.neg)
	}
	if a.isZero() {
		return zero(a.neg != b.neg)
	}
	words := quotientWords(a, b)
	qexp := a.wordExp() - b.wordExp() + 1
	lowest := 9 * (qexp - words) // the power of ten of the last digit kept
	return divideTo(a, b, lowest, mode, true)
}

// divideTo is a / b with its last digit at 10^lowest, rounded by mode
// as bigdecimal rounds a quotient, or truncated.
func divideTo(a, b dec, lowest int64, mode int, round bool) dec {
	// a / b = (ac × 10^ae) / (bc × 10^be); scale so the integer quotient's
	// units are 10^lowest.
	num := new(big.Int).Set(a.coef)
	den := new(big.Int).Set(b.coef)
	shift := a.exp - b.exp - lowest
	if shift >= 0 {
		num.Mul(num, pow10(shift))
	} else {
		den.Mul(den, pow10(-shift))
	}
	q, r := new(big.Int).QuoRem(num, den, new(big.Int))
	negative := a.neg != b.neg
	if round && r.Sign() != 0 {
		// bigdecimal estimates the next digit from the leading words of
		// the remainder and the divisor, in 32 bits; the estimate is
		// rough, and so is Ruby's last digit.
		rem := norm(false, r, min(a.exp, b.exp+lowest))
		v := nextDigitEstimate(rem, b)
		up := false
		switch mode {
		case roundUp:
			up = v != 0
		case roundHalfUp:
			up = v >= 5
		case roundHalfDown:
			up = v > 5
		case roundHalfEven:
			up = v > 5 || v == 5 && q.Bit(0) == 1
		case roundCeiling:
			up = v != 0 && !negative
		case roundFloor:
			up = v != 0 && negative
		}
		if up {
			q.Add(q, bigOne)
		}
	}
	return norm(negative && q.Sign() != 0, q, lowest)
}

// leadingWord is the first nine-digit word of a number as bigdecimal
// stores it: 0.w1 w2 ... × 1000000000^wordExp.
func leadingWord(d dec) uint64 {
	if d.isZero() {
		return 0
	}
	shift := d.exp - 9*(d.wordExp()-1)
	w := new(big.Int).Set(d.coef)
	if shift >= 0 {
		w.Mul(w, pow10(shift))
	} else {
		w.Quo(w, pow10(-shift))
	}
	return w.Uint64()
}

// nextDigitEstimate is bigdecimal's guess at the digit after a quotient:
// BASE × (remainder's leading word) / (divisor's leading word), cut to
// 32 bits, and its leading digit.
func nextDigitEstimate(rem, b dec) uint64 {
	r0, b0 := leadingWord(rem), leadingWord(b)
	if r0 == 0 || b0 == 0 {
		return 0
	}
	v := uint32(1000000000 * r0 / b0)
	return uint64(v) / 100000000
}

// divideSig is div(b, n): the quotient rounded to n significant digits.
func divideSig(a, b dec, n int64, mode int) dec {
	if n <= 0 {
		return divide(a, b, mode)
	}
	if !a.finite() || !b.finite() || b.isZero() || a.isZero() {
		return divide(a, b, mode)
	}
	// The quotient's leading digit is near 10^(ea - eb); keep n digits
	// and a few more, then round.
	lead := a.exponent() - b.exponent()
	q := divideTo(a, b, lead-n-3, roundDown, false)
	return roundSig(q, n, mode)
}

// floorDiv is a.div(b): the quotient rounded towards minus infinity.
func floorDiv(a, b dec) dec {
	as, bs, _ := aligned(a, b)
	q, r := new(big.Int).QuoRem(as, bs, new(big.Int))
	if r.Sign() != 0 && (r.Sign() < 0) != (bs.Sign() < 0) {
		q.Sub(q, bigOne)
	}
	return fromInt(q)
}

// modulo is a % b: a - b × floor(a / b).
func modulo(a, b dec) dec {
	q := floorDiv(a, b)
	return sub(a, mul(b, q))
}

// remainder is a - b × truncate(a / b).
func remainder(a, b dec) dec {
	as, bs, exp := aligned(a, b)
	r := new(big.Int).Rem(as, bs)
	return fromSigned(r, exp)
}

// powerInt is x ** n for an Integer n, exact for n >= 0; a negative n
// divides one by the power to the words bigdecimal computes, truncated.
func powerInt(x dec, n int64, mode int) dec {
	if x.isNaN() {
		return x
	}
	if n == 0 {
		return fromInt(bigOne)
	}
	if x.isInf() {
		if n < 0 {
			return zero(false)
		}
		return decInf(x.neg && n%2 == 1)
	}
	if x.isZero() {
		if n < 0 {
			return decInf(x.neg && n%2 == 1)
		}
		return zero(x.neg && n%2 == 1)
	}
	m := n
	if m < 0 {
		m = -m
	}
	coef := new(big.Int).Exp(x.coef, big.NewInt(m), nil)
	p := norm(x.neg && m%2 == 1, coef, x.exp*m)
	if n > 0 {
		return p
	}
	// The result's allocation in words follows bigdecimal's: ten digits
	// a word of x for each power, plus two words for the quotient.
	xwords := x.wordSpan()
	ywords := (xwords*10*(m+1) + 8) / 9
	words := ywords + 2 - 1
	qexp := 1 - p.wordExp() + 1
	one := fromInt(bigOne)
	return divideTo(one, p, 9*(qexp-words), mode, false)
}

// toFloat is to_f.
func (d dec) toFloat() float64 {
	switch d.special {
	case nan:
		return math.NaN()
	case posInf:
		return math.Inf(1)
	case negInf:
		return math.Inf(-1)
	}
	if d.isZero() {
		if d.neg {
			return math.Copysign(0, -1)
		}
		return 0
	}
	f, err := strconv.ParseFloat(d.engineering("", 0), 64)
	if err != nil {
		if d.exponent() > 0 {
			return math.Inf(d.sign())
		}
		return 0
	}
	return f
}

// toInt is the integer part.
func (d dec) toInt() *big.Int {
	if d.exp >= 0 {
		return new(big.Int).Mul(d.signedCoef(), pow10(d.exp))
	}
	q := new(big.Int).Quo(d.coef, pow10(-d.exp))
	if d.neg {
		q.Neg(q)
	}
	return q
}

// sqrt is the square root to n significant digits, correctly rounded
// down.
func sqrt(d dec, n int64) dec {
	if d.isZero() {
		return zero(false)
	}
	if n < 16 {
		n = 16
	}
	// sqrt(c × 10^e): make the exponent even, then an integer root with
	// 2n extra digits.
	coef := new(big.Int).Set(d.coef)
	exp := d.exp
	if exp%2 != 0 {
		coef.Mul(coef, bigTen)
		exp--
	}
	extra := 2*n + 4
	if extra%2 != 0 {
		extra++
	}
	coef.Mul(coef, pow10(extra))
	root := new(big.Int).Sqrt(coef)
	r := norm(false, root, (exp-extra)/2)
	return roundSig(r, n, roundDown)
}
