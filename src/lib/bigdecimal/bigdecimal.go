// Package bigdecimal is Ruby's bigdecimal library, built into cameo:
// BigDecimal, arbitrary-precision decimal numbers, with bigdecimal/util's
// to_d and bigdecimal/math's BigMath.
//
// It follows bigdecimal 3.1, the version Ruby 3.3 and 3.4 ship: the
// syntax BigDecimal() reads, to_s in its forms, exact addition,
// subtraction and multiplication, division to the digits bigdecimal
// computes, the rounding modes, limit and mode, the predicates, split,
// precision and scale, and the errors. A BigDecimal is a value with its
// digits as fields, a Hash key and a Set member; it is a Numeric, and an
// Integer or Float on the left of an operator coerces to it. cameo has no
// Rational, so to_r is left out, and the square root and BigMath give the
// digits asked for, correctly rounded, where Ruby pads them with its own
// extra digits.
package bigdecimal

import (
	"github.com/cameo-lang/cameo/src/str"
	"math/big"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class   = object.NewClass("BigDecimal", object.NumericClass)
	bigMath = object.NewModule("BigMath")
	// floatDomainError and domainError are Ruby's FloatDomainError and
	// Math::DomainError, found at install time.
	floatDomainError *object.Class
	domainError      *object.Class
)

const (
	fSign = iota
	fDigits
	fExponent
	fSpecial
)

// Settings are process-wide, as Ruby's are per thread.
var (
	limit     int64
	roundMode = roundHalfUp
	excFlags  int64
)

const (
	excNaN        = 2
	excInfinity   = 1
	excUnderflow  = 4
	excOverflow   = 1
	excZeroDivide = 16
	excAll        = 255
)

func readD(v object.Value) dec {
	f := v.Instance().Fields
	switch f[fSpecial].Str() {
	case "NaN":
		return decNaN()
	case "Infinity":
		return decInf(false)
	case "-Infinity":
		return decInf(true)
	}
	sign, _ := f[fSign].AsInt()
	e, _ := f[fExponent].AsInt()
	digits := f[fDigits].Str()
	if digits == "" {
		return zero(sign < 0)
	}
	coef, _ := new(big.Int).SetString(digits, 10)
	return dec{neg: sign < 0, coef: coef, exp: e - int64(len(digits))}
}

func build(d dec) object.Value {
	fields := make([]object.Value, 4)
	fields[fSign] = object.Int(int64(d.sign()))
	fields[fDigits] = object.NewString("", str.ASCII)
	fields[fExponent] = object.Int(0)
	fields[fSpecial] = object.NewString(d.plainSpecial(), str.UTF8)
	if d.finite() && !d.isZero() {
		fields[fDigits] = object.NewString(d.digits(), str.ASCII)
		fields[fExponent] = object.Int(d.exponent())
	}
	if d.special == nan {
		fields[fSign] = object.Int(0)
	}
	return object.InstanceValue(&object.Instance{Cls: class, Fields: fields})
}

func isBig(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls == class
}

// toDec reads an operand: a BigDecimal, an Integer or a Float.
func toDec(ctx object.Context, v object.Value) (dec, error) {
	switch {
	case isBig(v):
		return readD(v), nil
	case v.IsInteger():
		if i, ok := v.AsInt(); ok {
			return fromInt(big.NewInt(i)), nil
		}
		return fromInt(v.Big()), nil
	case v.Kind() == object.KindFloat:
		return fromFloat(v.Float(), 0, roundMode), nil
	}
	return dec{}, ctx.Raise(object.TypeError, "%s can't be coerced into BigDecimal", stdlib.KindName(v))
}

func domainErr(msg string) error { return &object.Exception{Cls: floatDomainError, Message: msg} }

// check applies the exception mode and the limit to a result.
func check(d dec, limited bool) (object.Value, error) {
	switch d.special {
	case nan:
		if excFlags&excNaN != 0 {
			return object.Value{}, domainErr("Computation results in 'NaN' (Not a Number)")
		}
	case posInf:
		if excFlags&excInfinity != 0 {
			return object.Value{}, domainErr("Computation results in 'Infinity'")
		}
	case negInf:
		if excFlags&excInfinity != 0 {
			return object.Value{}, domainErr("Computation results in '-Infinity'")
		}
	}
	if limited && limit > 0 {
		d = roundSig(d, limit, roundMode)
	}
	return build(d), nil
}

// modeArg reads a rounding mode: its number, or a Symbol or String.
func modeArg(ctx object.Context, v object.Value) (int, error) {
	if n, ok := v.AsInt(); ok {
		if n < roundUp || n > roundHalfEven {
			return 0, ctx.Raise(object.ArgumentError, "invalid rounding mode (%d)", n)
		}
		return int(n), nil
	}
	name := ""
	if v.Kind() == object.KindSymbol {
		name = v.Str()
	}
	switch name {
	case "up":
		return roundUp, nil
	case "down", "truncate":
		return roundDown, nil
	case "half_up", "default":
		return roundHalfUp, nil
	case "half_down":
		return roundHalfDown, nil
	case "half_even", "banker":
		return roundHalfEven, nil
	case "ceiling", "ceil":
		return roundCeiling, nil
	case "floor":
		return roundFloor, nil
	}
	if name == "" {
		return 0, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer", stdlib.KindName(v))
	}
	return 0, ctx.Raise(object.ArgumentError, "invalid rounding mode (%s)", name)
}

// integerOf is a finite decimal's integer part as a Value; a special
// value is a FloatDomainError.
func integerOf(d dec) (object.Value, error) {
	switch d.special {
	case nan:
		return object.Value{}, domainErr("Computation results in 'NaN' (Not a Number)")
	case posInf:
		return object.Value{}, domainErr("Computation results in 'Infinity'")
	case negInf:
		return object.Value{}, domainErr("Computation results in '-Infinity'")
	}
	return object.BigInt(d.toInt()), nil
}

// convert is BigDecimal(value, prec = nil, exception: true).
func convert(ctx object.Context, args []object.Value, kw *object.Map) (object.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
	}
	exception := true
	if kw != nil {
		if v, ok := kw.Get(object.Symbol("exception")); ok {
			exception = v.Truthy()
		}
	}
	fail := func(err error) (object.Value, error) {
		if !exception {
			return object.Nil, nil
		}
		return object.Value{}, err
	}
	v := args[0]
	var prec int64 = -1
	if len(args) == 2 && !args[1].IsNil() {
		n, err := stdlib.WantInt(ctx, args[1], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return fail(ctx.Raise(object.ArgumentError, "negative precision"))
		}
		prec = n
	}
	var d dec
	switch {
	case isBig(v):
		d = readD(v)
	case v.IsInteger():
		d, _ = toDec(ctx, v)
	case v.Kind() == object.KindFloat:
		if prec < 0 {
			return fail(ctx.Raise(object.ArgumentError, "can't omit precision for a Float."))
		}
		if prec > 16 {
			return fail(ctx.Raise(object.ArgumentError, "precision too large."))
		}
		d = fromFloat(v.Float(), prec, roundMode)
		return build(d), nil
	case v.IsString():
		var ok bool
		d, ok = parse(v.Str())
		if !ok {
			return fail(ctx.Raise(object.ArgumentError, "invalid value for BigDecimal(): %s", strconv.Quote(v.Str())))
		}
	case v.IsNil():
		return fail(ctx.Raise(object.TypeError, "can't convert nil into BigDecimal"))
	default:
		return fail(ctx.Raise(object.TypeError, "can't convert %s into BigDecimal", stdlib.KindName(v)))
	}
	if prec > 0 {
		d = roundSig(d, prec, roundMode)
	}
	return build(d), nil
}

// Install defines BigDecimal, BigMath and to_d. lib.Install calls it
// once.
func Install() {
	c := class
	object.DefineNative("BigDecimal", object.ClassValue(c))
	object.DefineNative("BigMath", object.ClassValue(bigMath))
	if v, ok := object.ObjectClass.Consts["FloatDomainError"]; ok {
		floatDomainError, _ = v.AsClass()
	} else {
		floatDomainError = object.NewClass("FloatDomainError", object.RangeError)
		object.DefineNative("FloatDomainError", object.ClassValue(floatDomainError))
	}
	if m, ok := object.ObjectClass.Consts["Math"]; ok {
		mc, _ := m.AsClass()
		if v, ok := mc.Consts["DomainError"]; ok {
			domainError, _ = v.AsClass()
		} else {
			domainError = object.NewClass("DomainError", object.ArgumentError)
			domainError.Lexical = mc
			mc.Consts["DomainError"] = object.ClassValue(domainError)
		}
	} else {
		domainError = object.ArgumentError
	}
	used := map[int64]bool{}
	taken := func(number int64) bool {
		if used[number] {
			return true
		}
		used[number] = true
		return false
	}
	for _, f := range []struct {
		name string
		typ  *object.FieldType
	}{{"sign", object.ScalarField("int")}, {"digits", object.ScalarField("string")}, {"exponent", object.ScalarField("int")}, {"special", object.ScalarField("string")}} {
		field := &object.Field{Name: f.name, Number: protogen.FieldNumber(f.name, taken), Type: f.typ, Owner: c}
		c.FieldIndex[f.name] = len(c.Fields)
		c.Fields = append(c.Fields, field)
	}
	for name, v := range map[string]int64{
		"ROUND_MODE": 256, "ROUND_UP": roundUp, "ROUND_DOWN": roundDown, "ROUND_HALF_UP": roundHalfUp, "ROUND_HALF_DOWN": roundHalfDown,
		"ROUND_CEILING": roundCeiling, "ROUND_FLOOR": roundFloor, "ROUND_HALF_EVEN": roundHalfEven,
		"SIGN_NaN": 0, "SIGN_POSITIVE_ZERO": 1, "SIGN_NEGATIVE_ZERO": -1, "SIGN_POSITIVE_FINITE": 2, "SIGN_NEGATIVE_FINITE": -2,
		"SIGN_POSITIVE_INFINITE": 3, "SIGN_NEGATIVE_INFINITE": -3,
		"EXCEPTION_ALL": excAll, "EXCEPTION_NaN": excNaN, "EXCEPTION_INFINITY": excInfinity, "EXCEPTION_UNDERFLOW": excUnderflow,
		"EXCEPTION_OVERFLOW": excOverflow, "EXCEPTION_ZERODIVIDE": excZeroDivide, "BASE": 1000000000,
	} {
		c.Consts[name] = object.Int(v)
	}
	c.Consts["VERSION"] = object.NewString("3.1.8", str.ASCII)
	c.Consts["INFINITY"] = build(decInf(false))
	c.Consts["NAN"] = build(decNaN())
	object.DefineFunction("BigDecimal", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return convert(ctx, args, kw)
	})
	installClassMethods()
	installArithmetic()
	installRounding()
	installQueries()
	installConversions()
	installUtil()
	installBigMath()
}

type decFn func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

func def(name string, arity int, f decFn) {
	class.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return f(ctx, readD(self), self, args, kw, block)
	})
}

func defS(name string, arity int, f object.NativeFn) { class.DefineSingleton(name, arity, f) }

func installClassMethods() {
	defS("limit", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		old := limit
		if len(args) > 0 && !args[0].IsNil() {
			n, err := stdlib.WantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			if n < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "argument must be positive")
			}
			limit = n
		}
		return object.Int(old), nil
	})
	defS("mode", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		which, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if which == 256 {
			if len(args) == 2 && !args[1].IsNil() {
				m, err := modeArg(ctx, args[1])
				if err != nil {
					return object.Value{}, err
				}
				roundMode = m
			}
			return object.Int(int64(roundMode)), nil
		}
		if which&excAll != which || which == 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "first argument for BigDecimal.mode invalid")
		}
		if len(args) == 2 && !args[1].IsNil() {
			if args[1].Kind() != object.KindBool {
				return object.Value{}, ctx.Raise(object.ArgumentError, "second argument must be true or false")
			}
			if args[1].Truthy() {
				excFlags |= which
			} else {
				excFlags &^= which
			}
		}
		return object.Int(excFlags & which), nil
	})
	save := func(name string, get func() int64, set func(int64)) {
		defS(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			old := get()
			defer set(old)
			return ctx.Yield(block)
		})
	}
	save("save_rounding_mode", func() int64 { return int64(roundMode) }, func(v int64) { roundMode = int(v) })
	save("save_exception_mode", func() int64 { return excFlags }, func(v int64) { excFlags = v })
	save("save_limit", func() int64 { return limit }, func(v int64) { limit = v })
	defS("double_fig", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(16), nil
	})
	defS("_load", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if i := strings.IndexByte(s, ':'); i >= 0 {
			s = s[i+1:]
		}
		d, ok := parse(s)
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "load failed: invalid character in the marshaled string")
		}
		return build(d), nil
	})
	defS("interpret_loosely", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return build(parseLoose(s)), nil
	})
}

func installArithmetic() {
	binary := func(name string, f func(a, b dec) dec) {
		def(name, 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			o, err := toDec(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return check(f(d, o), true)
		})
	}
	binary("+", add)
	binary("-", sub)
	binary("*", mul)
	divOp := func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if o.isZero() && d.finite() && excFlags&excZeroDivide != 0 {
			return object.Value{}, domainErr("Divide by zero")
		}
		return check(divide(d, o, roundMode), true)
	}
	def("/", 1, divOp)
	def("quo", 1, divOp)
	// add, sub and mult with a precision round the result to it; 0 is
	// the plain operator.
	withPrec := func(name string, f func(a, b dec) dec) {
		def(name, 2, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			o, err := toDec(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			n, err := stdlib.WantInt(ctx, args[1], "")
			if err != nil {
				return object.Value{}, err
			}
			r := f(d, o)
			if n > 0 {
				r = roundSig(r, n, roundMode)
			}
			return check(r, n == 0)
		})
	}
	withPrec("add", add)
	withPrec("sub", sub)
	withPrec("mult", mul)
	def("div", -1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if len(args) == 2 && !args[1].IsNil() {
			n, err := stdlib.WantInt(ctx, args[1], "")
			if err != nil {
				return object.Value{}, err
			}
			if n < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative precision")
			}
			if o.isZero() && d.finite() && excFlags&excZeroDivide != 0 {
				return object.Value{}, domainErr("Divide by zero")
			}
			return check(divideSig(d, o, n, roundMode), n == 0)
		}
		if o.isZero() {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		if !d.finite() || !o.finite() {
			q := divide(d, o, roundMode)
			if !q.finite() {
				return integerOf(q)
			}
			return object.BigInt(q.toInt()), nil
		}
		return object.BigInt(floorDiv(d, o).toInt()), nil
	})
	mod := func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if o.isZero() {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		if !d.finite() || o.isNaN() {
			return check(decNaN(), false)
		}
		if o.isInf() {
			// A finite number is its own remainder by an infinity.
			return self, nil
		}
		return check(modulo(d, o), true)
	}
	def("%", 1, mod)
	def("modulo", 1, mod)
	def("remainder", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if o.isZero() {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		if !d.finite() || o.isNaN() {
			return check(decNaN(), false)
		}
		if o.isInf() {
			return self, nil
		}
		return check(remainder(d, o), true)
	})
	def("divmod", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if o.isZero() {
			return object.Value{}, ctx.Raise(object.ZeroDivisionError, "divided by 0")
		}
		if !d.finite() || o.isNaN() {
			return object.NewList(build(decNaN()), build(decNaN())), nil
		}
		if o.isInf() {
			return object.NewList(build(zero(false)), self), nil
		}
		q := floorDiv(d, o)
		return object.NewList(build(q), build(sub(d, mul(o, q)))), nil
	})
	def("fdiv", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		// Numeric#fdiv is Float(self) / other: a Float, or a BigDecimal
		// when other is one, through coerce.
		if isBig(args[0]) {
			return check(divide(fromFloat(d.toFloat(), 0, roundMode), o, roundMode), true)
		}
		return object.Float(d.toFloat() / o.toFloat()), nil
	})
	power := func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		var prec int64
		if len(args) == 2 && !args[1].IsNil() {
			var err error
			if prec, err = stdlib.WantInt(ctx, args[1], ""); err != nil {
				return object.Value{}, err
			}
			if prec < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative precision")
			}
		}
		e := args[0]
		if isBig(e) {
			ed := readD(e)
			if ed.finite() && ed.exp >= 0 {
				e = object.BigInt(ed.toInt())
			}
		}
		if n, ok := e.AsInt(); ok {
			r := powerInt(d, n, roundMode)
			if prec > 0 {
				r = roundSig(r, prec, roundMode)
			}
			return check(r, prec == 0)
		}
		ed, err := toDec(ctx, e)
		if err != nil {
			return object.Value{}, err
		}
		if !d.finite() || !ed.finite() {
			return check(decNaN(), false)
		}
		if d.isZero() {
			// 0 ** x is 0 for a positive x and Infinity for a negative one,
			// as Ruby's is.
			if ed.neg {
				return check(decInf(false), false)
			}
			return check(zero(false), prec == 0)
		}
		if d.neg {
			return object.Value{}, &object.Exception{Cls: domainError, Message: "Zero or negative argument for log"}
		}
		digits := prec
		if digits == 0 {
			digits = d.wordSpan()*9 + 16
		}
		return check(powerFloat(d, ed, digits), prec == 0)
	}
	def("**", 1, power)
	def("power", -1, power)
	def("-@", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return build(neg(d)), nil
	})
	def("+@", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def("abs", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if d.neg || d.special == negInf {
			return build(neg(d)), nil
		}
		return self, nil
	})
	def("abs2", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return check(mul(d, d), true)
	})
	def("sqrt", -1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := int64(0)
		if len(args) > 0 && !args[0].IsNil() {
			var err error
			if n, err = stdlib.WantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
			if n < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative precision")
			}
		}
		switch {
		case d.isNaN():
			return object.Value{}, domainErr("sqrt of 'NaN'(Not a Number)")
		case d.neg && !d.isZero():
			return object.Value{}, domainErr("sqrt of negative value")
		case d.special == posInf:
			return self, nil
		}
		return build(sqrt(d, n)), nil
	})
	// Comparison.
	compare := func(ctx object.Context, d dec, other object.Value) (int, bool, error) {
		var o dec
		switch {
		case isBig(other) || other.IsInteger() || other.Kind() == object.KindFloat:
			var err error
			if o, err = toDec(ctx, other); err != nil {
				return 0, false, err
			}
		default:
			return 0, false, nil
		}
		c, ok := cmp(d, o)
		return c, ok, nil
	}
	def("<=>", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, ok, err := compare(ctx, d, args[0])
		if err != nil || !ok {
			return object.Nil, err
		}
		return object.Int(int64(c)), nil
	})
	def("==", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, ok, err := compare(ctx, d, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(ok && c == 0), nil
	})
	def("eql?", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, ok, err := compare(ctx, d, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(ok && c == 0), nil
	})
	def("===", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c, ok, err := compare(ctx, d, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(ok && c == 0), nil
	})
	rel := func(name string, test func(c int) bool) {
		def(name, 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			c, ok, err := compare(ctx, d, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if !ok {
				if isBig(args[0]) || args[0].IsInteger() || args[0].Kind() == object.KindFloat {
					return object.False, nil
				}
				return object.Value{}, ctx.Raise(object.ArgumentError, "comparison of BigDecimal with %s failed", stdlib.KindName(args[0]))
			}
			return object.Bool(test(c)), nil
		})
	}
	rel("<", func(c int) bool { return c < 0 })
	rel("<=", func(c int) bool { return c <= 0 })
	rel(">", func(c int) bool { return c > 0 })
	rel(">=", func(c int) bool { return c >= 0 })
	def("coerce", 1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o, err := toDec(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(build(o), self), nil
	})
}

func installRounding() {
	// round(n = nil, mode = nil): an Integer without n, a BigDecimal with.
	def("round", -1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		mode := roundMode
		if len(args) == 2 {
			var err error
			if mode, err = modeArg(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		if len(args) == 0 {
			return integerOf(roundAt(d, 0, mode))
		}
		if len(args) == 1 && (args[0].Kind() == object.KindSymbol || args[0].IsString()) {
			var err error
			if mode, err = modeArg(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
			return integerOf(roundAt(d, 0, mode))
		}
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if len(args) == 1 && n < 1 {
			return integerOf(roundAt(d, n, mode))
		}
		return check(roundAt(d, n, mode), false)
	})
	rounding := func(name string, mode int) {
		def(name, -1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
			}
			if len(args) == 0 {
				return integerOf(roundAt(d, 0, mode))
			}
			n, err := stdlib.WantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			return check(roundAt(d, n, mode), false)
		})
	}
	rounding("floor", roundFloor)
	rounding("ceil", roundCeiling)
	rounding("truncate", roundDown)
	def("fix", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return build(roundAt(d, 0, roundDown)), nil
	})
	def("frac", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !d.finite() {
			return self, nil
		}
		return build(sub(d, roundAt(d, 0, roundDown))), nil
	})
}

func installQueries() {
	intQ := func(name string, f func(d dec) int64) {
		def(name, 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(f(d)), nil
		})
	}
	intQ("precision", func(d dec) int64 { return d.precision() })
	intQ("scale", func(d dec) int64 { return d.scale() })
	intQ("n_significant_digits", func(d dec) int64 {
		if !d.finite() || d.isZero() {
			return 0
		}
		return int64(d.ndigits())
	})
	intQ("exponent", func(d dec) int64 { return d.exponent() })
	intQ("sign", func(d dec) int64 {
		switch d.special {
		case nan:
			return 0
		case posInf:
			return 3
		case negInf:
			return -3
		}
		if d.isZero() {
			return int64(d.sign())
		}
		return int64(2 * d.sign())
	})
	def("precision_scale", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(object.Int(d.precision()), object.Int(d.scale())), nil
	})
	boolQ := func(name string, f func(d dec) bool) {
		def(name, 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Bool(f(d)), nil
		})
	}
	boolQ("zero?", func(d dec) bool { return d.isZero() })
	boolQ("finite?", func(d dec) bool { return d.finite() })
	boolQ("nan?", func(d dec) bool { return d.isNaN() })
	boolQ("positive?", func(d dec) bool { return d.special == posInf || d.finite() && !d.isZero() && !d.neg })
	boolQ("negative?", func(d dec) bool { return d.special == negInf || d.finite() && !d.isZero() && d.neg })
	boolQ("integer?", func(d dec) bool { return false })
	boolQ("frozen?", func(d dec) bool { return true })
	def("nonzero?", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if d.isZero() {
			return object.Nil, nil
		}
		return self, nil
	})
	def("infinite?", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch d.special {
		case posInf:
			return object.Int(1), nil
		case negInf:
			return object.Int(-1), nil
		}
		return object.Nil, nil
	})
	def("split", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sign, digits, e := int64(d.sign()), "0", int64(0)
		switch {
		case d.isNaN():
			sign, digits = 0, "NaN"
		case d.isInf():
			digits = "Infinity"
		case !d.isZero():
			digits, e = d.digits(), d.exponent()
		}
		return object.NewList(object.Int(sign), object.NewString(digits, str.UTF8), object.Int(10), object.Int(e)), nil
	})
	for _, name := range []string{"clone", "dup"} {
		def(name, 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return self, nil
		})
	}
}

func installConversions() {
	toS := func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		prefix, group, plain := "", 0, false
		if len(args) > 0 && !args[0].IsNil() {
			if n, ok := args[0].AsInt(); ok {
				group = int(n)
			} else {
				f, err := stdlib.WantString(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				if strings.HasPrefix(f, "+") || strings.HasPrefix(f, " ") {
					prefix, f = f[:1], f[1:]
				}
				digits := strings.TrimRight(f, "EF")
				if digits != "" {
					n, err := strconv.Atoi(digits)
					if err != nil {
						return object.Value{}, ctx.Raise(object.ArgumentError, "invalid format: %s", strconv.Quote(f))
					}
					group = n
				}
				plain = strings.HasSuffix(f, "F")
			}
		}
		if plain {
			return object.NewString(d.plain(prefix, group), str.UTF8), nil
		}
		return object.NewString(d.engineering(prefix, group), str.UTF8), nil
	}
	def("to_s", -1, toS)
	def("inspect", 0, toS)
	def("to_digits", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString(d.plain("", 0), str.ASCII), nil
	})
	def("to_f", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Float(d.toFloat()), nil
	})
	for _, name := range []string{"to_i", "to_int"} {
		def(name, 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return integerOf(d)
		})
	}
	def("to_d", 0, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	def("_dump", -1, func(ctx object.Context, d dec, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		words := (int64(d.ndigits())+8)/9 + 1
		return object.NewString(strconv.FormatInt(words*9, 10)+":"+d.engineering("", 0), str.ASCII), nil
	})
}

// installUtil defines bigdecimal/util's to_d on String, Integer, Float
// and nil.
func installUtil() {
	object.StringClass.Define("to_d", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return build(parseLoose(self.Str())), nil
	})
	object.IntegerClass.Define("to_d", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d, err := toDec(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return build(d), nil
	})
	object.FloatClass.Define("to_d", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		prec := int64(0)
		if len(args) > 0 && !args[0].IsNil() {
			var err error
			if prec, err = stdlib.WantInt(ctx, args[0], ""); err != nil {
				return object.Value{}, err
			}
			if prec < 0 {
				return object.Value{}, ctx.Raise(object.ArgumentError, "negative precision")
			}
		}
		return build(fromFloat(self.Float(), prec, roundMode)), nil
	})
	object.NilClass.Define("to_d", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return build(zero(false)), nil
	})
}

// installBigMath defines BigMath's functions: each to the digits asked
// for, correctly rounded.
func installBigMath() {
	m := bigMath
	precArg := func(ctx object.Context, v object.Value) (int64, error) {
		n, err := stdlib.WantInt(ctx, v, "")
		if err != nil {
			return 0, err
		}
		if n <= 0 {
			return 0, ctx.Raise(object.ArgumentError, "Zero or negative precision for %s", "BigMath")
		}
		return n, nil
	}
	constant := func(name string, f func(prec uint) *big.Float) {
		m.DefineSingleton(name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n, err := precArg(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return build(fromBigFloat(f(bitsFor(n)), n)), nil
		})
	}
	constant("PI", piF)
	constant("E", func(prec uint) *big.Float { return expF(floatInt(prec, 1), prec) })
	// fn defines a function of BigMath: check refuses an argument, and inf
	// is the value for an infinite one, nil where it is NaN.
	fn := func(name string, f func(x *big.Float, prec uint) *big.Float, check func(ctx object.Context, d dec) error, inf func(neg bool, n int64) dec) {
		m.DefineSingleton(name, 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			d, err := toDec(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			n, err := precArg(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			if check != nil {
				if err := check(ctx, d); err != nil {
					return object.Value{}, err
				}
			}
			if d.isInf() && inf != nil {
				return build(inf(d.special == negInf, n)), nil
			}
			if !d.finite() {
				return build(decNaN()), nil
			}
			prec := bitsFor(n)
			return build(fromBigFloat(f(d.toBigFloat(prec+32), prec), n)), nil
		})
	}
	infinity := func(neg bool, n int64) dec { return decInf(false) }
	fn("sqrt", func(x *big.Float, prec uint) *big.Float { return newF(prec).Sqrt(x) }, func(ctx object.Context, d dec) error {
		if d.neg && !d.isZero() {
			return domainErr("sqrt of negative value")
		}
		return nil
	}, infinity)
	fn("sin", func(x *big.Float, prec uint) *big.Float { return sinCosF(x, prec, false) }, nil, nil)
	fn("cos", func(x *big.Float, prec uint) *big.Float { return sinCosF(x, prec, true) }, nil, nil)
	fn("atan", atanF, nil, func(negative bool, n int64) dec {
		// atan of an infinity is a half pi, to the digits asked for.
		prec := bitsFor(n)
		half := fromBigFloat(newF(prec).Quo(piF(prec), big.NewFloat(2)), n)
		if negative {
			return neg(half)
		}
		return half
	})
	fn("exp", expF, nil, func(neg bool, n int64) dec {
		if neg {
			return zero(false)
		}
		return decInf(false)
	})
	fn("log", logF, func(ctx object.Context, d dec) error {
		if d.neg || d.isZero() {
			return &object.Exception{Cls: domainError, Message: "Zero or negative argument for log"}
		}
		return nil
	}, infinity)
}
