package matrix

import (
	"fmt"
	"math"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

const fElems = 0

func readV(v object.Value) []object.Value { return v.Instance().Fields[fElems].List().Elems }

func newV(elems []object.Value) object.Value {
	return object.InstanceValue(&object.Instance{Cls: vectorClass, Fields: []object.Value{object.NewList(elems...)}})
}

func zeroVector(msg string) error { return &object.Exception{Cls: errZeroVec, Message: msg} }

type vectorFn func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

func defV(name string, arity int, f vectorFn) {
	vectorClass.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return f(ctx, readV(self), self, args, kw, block)
	})
}

// vectorArg is the elements of a Vector argument.
func vectorArg(ctx object.Context, v object.Value) ([]object.Value, error) {
	if !isVector(v) {
		return nil, ctx.Raise(object.TypeError, "Expected a Vector, got a %s", stdlib.KindName(v))
	}
	return readV(v), nil
}

// norm is the Euclidean length as a Float.
func norm(ctx object.Context, elems []object.Value) (float64, error) {
	sum := 0.0
	for _, e := range elems {
		f, ok := e.AsNumber()
		if !ok {
			return 0, ctx.Raise(object.TypeError, "%s can't be coerced into Float", stdlib.KindName(e))
		}
		sum += f * f
	}
	return math.Sqrt(sum), nil
}

func mapElems(ctx object.Context, elems []object.Value, f func(object.Value) (object.Value, error)) (object.Value, error) {
	out := make([]object.Value, len(elems))
	for i, e := range elems {
		v, err := f(e)
		if err != nil {
			return object.Value{}, err
		}
		out[i] = v
	}
	return newV(out), nil
}

func columnOf(elems []object.Value) matrix {
	rows := make([][]object.Value, len(elems))
	for i, v := range elems {
		rows[i] = []object.Value{v}
	}
	return matrix{rows: rows, cols: 1}
}

// independent says whether vectors of one size are linearly independent:
// the rank of the matrix of their rows is their count.
func independent(ctx object.Context, vs []object.Value) (bool, error) {
	if len(vs) == 0 {
		return true, nil
	}
	rows := make([][]object.Value, len(vs))
	for i, v := range vs {
		elems, err := vectorArg(ctx, v)
		if err != nil {
			return false, err
		}
		if len(elems) != len(readV(vs[0])) {
			return false, dimError()
		}
		rows[i] = elems
	}
	r, err := matrix{rows: rows, cols: len(rows[0])}.rank(ctx)
	return r == len(vs), err
}

func installVector() {
	c := vectorClass
	c.DefineSingleton("[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return newV(append([]object.Value(nil), args...)), nil
	})
	c.DefineSingleton("elements", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		elems, err := listArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return newV(elems), nil
	})
	c.DefineSingleton("zero", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid size (%d for 0..)", n)
		}
		elems := make([]object.Value, n)
		for i := range elems {
			elems[i] = object.Int(0)
		}
		return newV(elems), nil
	})
	c.DefineSingleton("basis", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var sizeArg, indexArg object.Value
		if kw != nil {
			sizeArg, _ = kw.Get(object.Symbol("size"))
			indexArg, _ = kw.Get(object.Symbol("index"))
		}
		if sizeArg.IsZero() || indexArg.IsZero() {
			return object.Value{}, ctx.Raise(object.ArgumentError, "Vector.basis takes size: and index:")
		}
		size, err := intArg(ctx, sizeArg)
		if err != nil {
			return object.Value{}, err
		}
		index, err := intArg(ctx, indexArg)
		if err != nil {
			return object.Value{}, err
		}
		if size < 1 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid size (%d for 1..)", size)
		}
		if index < 0 || index >= size {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid index (%d for 0...%d)", index, size)
		}
		elems := make([]object.Value, size)
		for i := range elems {
			elems[i] = object.Int(0)
		}
		elems[index] = object.Int(1)
		return newV(elems), nil
	})
	c.DefineSingleton("independent?", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ok, err := independent(ctx, args)
		return object.Bool(ok), err
	})
	defV("size", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(elems))), nil
	})
	index := func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		// An Integer, a Range or a start and a length, as Array#[] takes.
		return ctx.Call(self.Instance().Fields[fElems], "[]", args...)
	}
	defV("[]", 1, index)
	defV("element", 1, index)
	defV("component", 1, index)
	defV("[]=", 2, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 {
			i += len(elems)
		}
		if i < 0 || i >= len(elems) {
			return object.Value{}, ctx.Raise(object.IndexError, "index %d outside of vector bounds", i)
		}
		out := append([]object.Value(nil), elems...)
		out[i] = args[1]
		return newV(out), nil
	})
	defV("to_a", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.Instance().Fields[fElems], nil
	})
	defV("to_matrix", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		m := columnOf(elems)
		return buildM(m.rows, 1), nil
	})
	defV("covector", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return buildM([][]object.Value{append([]object.Value(nil), elems...)}, len(elems)), nil
	})
	defV("each", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, e := range elems {
					if err := yield(e); err != nil {
						return err
					}
				}
				return nil
			}, Size: func(ctx object.Context) (object.Value, error) { return object.Int(int64(len(elems))), nil }}), nil
		}
		for _, e := range elems {
			if _, err := ctx.Yield(block, e); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	defV("each2", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := pairArg(ctx, args[0], len(elems))
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each2", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for i, e := range elems {
					if err := yield(object.NewList(e, other[i])); err != nil {
						return err
					}
				}
				return nil
			}, Size: func(ctx object.Context) (object.Value, error) { return object.Int(int64(len(elems))), nil }}), nil
		}
		for i, e := range elems {
			if _, err := ctx.Yield(block, e, other[i]); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	collect := func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.ArgumentError, "collect needs a block")
		}
		return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return ctx.Yield(block, e) })
	}
	defV("collect", 0, collect)
	defV("map", 0, collect)
	collect2 := func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := pairArg(ctx, args[0], len(elems))
		if err != nil {
			return object.Value{}, err
		}
		out := make([]object.Value, len(elems))
		for i, e := range elems {
			if out[i], err = ctx.Yield(block, e, other[i]); err != nil {
				return object.Value{}, err
			}
		}
		return object.NewList(out...), nil
	}
	defV("collect2", 1, collect2)
	defV("map2", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		list, err := collect2(ctx, elems, self, args, kw, block)
		if err != nil {
			return object.Value{}, err
		}
		return newV(list.List().Elems), nil
	})
	// Arithmetic.
	addSubV := func(name string, f func(object.Context, object.Value, object.Value) (object.Value, error)) vectorFn {
		return func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			o := args[0]
			switch {
			case isVector(o):
				other := readV(o)
				if len(other) != len(elems) {
					return object.Value{}, dimError()
				}
				out := make([]object.Value, len(elems))
				for i, e := range elems {
					v, err := f(ctx, e, other[i])
					if err != nil {
						return object.Value{}, err
					}
					out[i] = v
				}
				return newV(out), nil
			case isMatrix(o):
				col := columnOf(elems)
				return addSub(ctx, col, buildM(col.rows, 1), o, name, f)
			}
			return object.Value{}, notDefined(name, self, o)
		}
	}
	defV("+", 1, addSubV("+", add))
	defV("-", 1, addSubV("-", sub))
	defV("*", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o := args[0]
		switch {
		case isNumeric(o):
			return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return mul(ctx, e, o) })
		case isMatrix(o):
			return columnOf(elems).times(ctx, readM(o))
		}
		return object.Value{}, notDefined("*", self, o)
	})
	defV("/", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o := args[0]
		if isNumeric(o) {
			return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return div(ctx, e, o) })
		}
		return object.Value{}, notDefined("/", self, o)
	})
	defV("-@", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return ctx.Call(e, "-@") })
	})
	defV("+@", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	inner := func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := vectorArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if len(other) != len(elems) {
			return object.Value{}, dimError()
		}
		return dot(ctx, elems, other)
	}
	defV("inner_product", 1, inner)
	defV("dot", 1, inner)
	cross := func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n := len(elems)
		if n < 2 {
			return object.Value{}, &object.Exception{Cls: errUndef, Message: fmt.Sprintf("cross product is not defined on vectors of dimension %d", n)}
		}
		if len(args) != n-2 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "wrong number of arguments (%d for %d)", len(args), n-2)
		}
		others := make([][]object.Value, len(args))
		for i, a := range args {
			o, err := vectorArg(ctx, a)
			if err != nil {
				return object.Value{}, err
			}
			if len(o) != n {
				return object.Value{}, dimError()
			}
			others[i] = o
		}
		switch n {
		case 2:
			neg, err := ctx.Call(elems[1], "-@")
			if err != nil {
				return object.Value{}, err
			}
			return newV([]object.Value{neg, elems[0]}), nil
		case 3:
			v, e := others[0], elems
			comp := func(a, b, c, d object.Value) (object.Value, error) {
				ab, err := mul(ctx, a, b)
				if err != nil {
					return object.Value{}, err
				}
				cd, err := mul(ctx, c, d)
				if err != nil {
					return object.Value{}, err
				}
				return sub(ctx, ab, cd)
			}
			x, err := comp(v[2], e[1], v[1], e[2])
			if err != nil {
				return object.Value{}, err
			}
			y, err := comp(v[0], e[2], v[2], e[0])
			if err != nil {
				return object.Value{}, err
			}
			z, err := comp(v[1], e[0], v[0], e[1])
			if err != nil {
				return object.Value{}, err
			}
			return newV([]object.Value{x, y, z}), nil
		}
		// In general: the cofactors of the last row of the matrix whose rows
		// are the vectors and a row of basis vectors.
		rows := append([][]object.Value{elems}, others...)
		rows = append(rows, make([]object.Value, n))
		m := matrix{rows: rows, cols: n}
		out := make([]object.Value, n)
		for i := range out {
			c, err := m.cofactor(ctx, n-1, i)
			if err != nil {
				return object.Value{}, err
			}
			out[i] = c
		}
		return newV(out), nil
	}
	defV("cross_product", -1, cross)
	defV("cross", -1, cross)
	for _, name := range []string{"norm", "magnitude", "r"} {
		defV(name, 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n, err := norm(ctx, elems)
			return object.Float(n), err
		})
	}
	defV("normalize", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := norm(ctx, elems)
		if err != nil {
			return object.Value{}, err
		}
		if n == 0 {
			return object.Value{}, zeroVector("Zero vectors can not be normalized")
		}
		return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return div(ctx, e, object.Float(n)) })
	})
	defV("angle_with", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		other, err := vectorArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if len(other) != len(elems) {
			return object.Value{}, dimError()
		}
		a, err := norm(ctx, elems)
		if err != nil {
			return object.Value{}, err
		}
		b, err := norm(ctx, other)
		if err != nil {
			return object.Value{}, err
		}
		prod := a * b
		if prod == 0 {
			return object.Value{}, zeroVector("Can't get angle of zero vector")
		}
		d, err := dot(ctx, elems, other)
		if err != nil {
			return object.Value{}, err
		}
		df, _ := d.AsNumber()
		if math.Abs(df) >= prod {
			if df > 0 {
				return object.Int(0), nil
			}
			return object.Float(math.Pi), nil
		}
		return object.Float(math.Acos(df / prod)), nil
	})
	defV("zero?", 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for _, e := range elems {
			z, err := isZero(ctx, e)
			if err != nil || !z {
				return object.False, err
			}
		}
		return object.True, nil
	})
	defV("independent?", -1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ok, err := independent(ctx, append([]object.Value{self}, args...))
		return object.Bool(ok), err
	})
	defV("round", -1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return mapElems(ctx, elems, func(e object.Value) (object.Value, error) { return ctx.Call(e, "round", args...) })
	})
	defV("==", 1, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isVector(args[0]) {
			return object.False, nil
		}
		eq, err := ctx.Equal(self.Instance().Fields[fElems], args[0].Instance().Fields[fElems])
		return object.Bool(eq), err
	})
	render := func(inspect bool) vectorFn {
		return func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			parts := make([]string, len(elems))
			for i, e := range elems {
				s, err := renderElem(ctx, e, inspect)
				if err != nil {
					return object.Value{}, err
				}
				parts[i] = s
			}
			return object.NewString("Vector["+strings.Join(parts, ", ")+"]", object.UTF8), nil
		}
	}
	defV("to_s", 0, render(false))
	defV("inspect", 0, render(true))
	for _, name := range []string{"conj", "conjugate", "real"} {
		defV(name, 0, func(ctx object.Context, elems []object.Value, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return self, nil
		})
	}
}

// pairArg is the other side of each2 and collect2: a Vector or an Array
// of the same size.
func pairArg(ctx object.Context, v object.Value, size int) ([]object.Value, error) {
	var other []object.Value
	if isVector(v) {
		other = readV(v)
	} else {
		var err error
		if other, err = stdlib.ToList(ctx, v); err != nil {
			return nil, err
		}
	}
	if len(other) != size {
		return nil, dimError()
	}
	return other, nil
}
