package matrix

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
	"github.com/cameo-lang/cameo/src/str"
)

// scalarClass is Matrix::Scalar, what Matrix#coerce and Vector#coerce
// give a number on the left of a matrix or a vector: Integer#* asks the
// matrix to coerce it, gets [Scalar, matrix], and calls Scalar#*. A
// Scalar holds its number and takes the four operators and ** as Ruby's
// does: with a number it stays a Scalar, with a matrix or a vector it
// multiplies, divides by the inverse, or refuses as Ruby refuses.
var scalarClass = object.NewClass("Scalar", object.ObjectClass)

func newScalar(n object.Value) object.Value {
	return object.InstanceValue(&object.Instance{Cls: scalarClass, Fields: []object.Value{n}})
}

func scalarOf(v object.Value) object.Value { return v.Instance().Fields[0] }

// coerce is Matrix#coerce and Vector#coerce: a number becomes a Scalar,
// anything else cannot be coerced.
func coerce(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if !isNumeric(args[0]) {
		return object.Value{}, ctx.Raise(object.TypeError, "%s can't be coerced into %s", stdlib.KindName(self), stdlib.KindName(args[0]))
	}
	return object.NewList(newScalar(args[0]), self), nil
}

func installScalar() {
	scalarClass.Lexical = matrixClass
	matrixClass.Consts["Scalar"] = object.ClassValue(scalarClass)
	scalarClass.DefineSingleton("new", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return newScalar(args[0]), nil
	})
	scalarClass.Define("value", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return scalarOf(self), nil
	})
	scalarClass.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.Inspect(scalarOf(self))
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString("#<Matrix::Scalar value="+s+">", str.UTF8), nil
	})
	matrixClass.Define("coerce", 1, coerce)
	vectorClass.Define("coerce", 1, coerce)
	// A scalar's operator with a number is the number's; with a matrix
	// or a vector, what Ruby's Scalar defines.
	for _, name := range []string{"+", "-", "*", "/", "**"} {
		name := name
		scalarClass.Define(name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n, o := scalarOf(self), args[0]
			switch {
			case isNumeric(o):
				r, err := op(ctx, n, name, o)
				if err != nil {
					return object.Value{}, err
				}
				return newScalar(r), nil
			case isMatrix(o) && name == "*":
				return readM(o).mapped(func(v object.Value, i, j int) (object.Value, error) { return mul(ctx, n, v) })
			case isVector(o) && name == "*":
				return mapElems(ctx, readV(o), func(e object.Value) (object.Value, error) { return mul(ctx, n, e) })
			case isMatrix(o) && name == "/":
				inv, err := readM(o).inverse(ctx)
				if err != nil {
					return object.Value{}, err
				}
				return readM(inv).mapped(func(v object.Value, i, j int) (object.Value, error) { return mul(ctx, n, v) })
			case isMatrix(o) && name == "**":
				return object.Value{}, notImplemented(name, n, o)
			case isMatrix(o) || isVector(o):
				return object.Value{}, notDefined(name, n, o)
			}
			shown, err := ctx.Inspect(o)
			if err != nil {
				return object.Value{}, err
			}
			return object.Value{}, ctx.Raise(object.TypeError, "%s can't be coerced into Matrix::Scalar", shown)
		})
	}
}
