package stdlib

import (
	"math"

	"github.com/radiospiel/cameo/src/object"
)

// installMath defines Math's functions and its constants PI and E. A
// function outside its domain raises Math::DomainError, as in Ruby.
func installMath() {
	m := object.MathModule
	m.Consts["PI"] = object.Float(math.Pi)
	m.Consts["E"] = object.Float(math.E)
	domainError := object.MathDomainError
	domainError.Lexical = m
	m.Consts["DomainError"] = object.ClassValue(domainError)

	// in reports whether a function's argument lies in its domain.
	unary := func(name string, f func(float64) float64, in func(float64) bool) {
		m.DefineSingleton(name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			x, err := wantFloat(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if in != nil && !in(x) {
				return object.Value{}, ctx.Raise(domainError, "Numerical argument is out of domain - %s", name)
			}
			return object.Float(f(x)), nil
		})
	}
	nonNegative := func(x float64) bool { return x >= 0 || math.IsNaN(x) }
	unit := func(x float64) bool { return -1 <= x && x <= 1 || math.IsNaN(x) }
	unary("sqrt", math.Sqrt, nonNegative)
	unary("cbrt", math.Cbrt, nil)
	unary("sin", math.Sin, nil)
	unary("cos", math.Cos, nil)
	unary("tan", math.Tan, nil)
	unary("asin", math.Asin, unit)
	unary("acos", math.Acos, unit)
	unary("atan", math.Atan, nil)
	unary("sinh", math.Sinh, nil)
	unary("cosh", math.Cosh, nil)
	unary("tanh", math.Tanh, nil)
	unary("exp", math.Exp, nil)
	unary("log2", math.Log2, nonNegative)
	unary("log10", math.Log10, nonNegative)
	binary := func(name string, f func(a, b float64) float64) {
		m.DefineSingleton(name, 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			a, err := wantFloat(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			b, err := wantFloat(ctx, args[1])
			if err != nil {
				return object.Value{}, err
			}
			return object.Float(f(a, b)), nil
		})
	}
	binary("atan2", math.Atan2)
	binary("hypot", math.Hypot)
	// log takes an optional base.
	m.DefineSingleton("log", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		x, err := wantFloat(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if !nonNegative(x) {
			return object.Value{}, ctx.Raise(domainError, "Numerical argument is out of domain - %s", "log")
		}
		if len(args) == 1 {
			return object.Float(math.Log(x)), nil
		}
		base, err := wantFloat(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		if !nonNegative(base) {
			return object.Value{}, ctx.Raise(domainError, "Numerical argument is out of domain - %s", "log")
		}
		return object.Float(math.Log(x) / math.Log(base)), nil
	})
}

// wantFloat reads a number as a float, as Math's functions take one.
func wantFloat(ctx object.Context, v object.Value) (float64, error) {
	if f, ok := v.AsNumber(); ok {
		return f, nil
	}
	return 0, ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(v))
}
