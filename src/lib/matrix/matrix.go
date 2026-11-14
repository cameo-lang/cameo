// Package matrix is Ruby's matrix library, built into cameo: Matrix and
// Vector, with their construction, arithmetic, decompositions of the
// determinant, the inverse and the rank, the predicates, and their errors.
//
// It follows matrix 0.4.2, the version Ruby 3.4 ships. A Matrix and a
// Vector are values with the elements as their fields, so they are Hash
// keys and Set members and compare by their elements; []= returns the
// changed copy. The element arithmetic is cameo's own, so Integer,
// Float and big Integer elements behave as Ruby's do, and the determinant
// and rank are Ruby's fraction-free algorithms. cameo has no Rational, so
// where Ruby's inverse of an Integer matrix has Rationals, cameo's has
// Floats, computed exactly first.
package matrix

import (
	"fmt"
	"github.com/cameo-lang/cameo/src/str"
	"slices"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	matrixClass = object.NewClass("Matrix", object.ObjectClass)
	vectorClass = object.NewClass("Vector", object.ObjectClass)
	errorsMod   = object.NewModule("ExceptionForMatrix")
	errDim      = object.NewClass("ErrDimensionMismatch", object.StandardError)
	errRegular  = object.NewClass("ErrNotRegular", object.StandardError)
	errUndef    = object.NewClass("ErrOperationNotDefined", object.StandardError)
	errUnimpl   = object.NewClass("ErrOperationNotImplemented", object.StandardError)
	errZeroVec  = object.NewClass("ZeroVectorError", object.StandardError)
)

const (
	fRows = iota
	fCols
)

// matrix is a Matrix as Go reads its fields: the rows, and the column
// count, which the rows do not tell when there are none.
type matrix struct {
	rows [][]object.Value
	cols int
}

func (m matrix) square() bool { return len(m.rows) == m.cols }

func readM(v object.Value) matrix {
	f := v.Instance().Fields
	cols, _ := f[fCols].AsInt()
	outer := f[fRows].List().Elems
	rows := make([][]object.Value, len(outer))
	for i, r := range outer {
		rows[i] = r.List().Elems
	}
	return matrix{rows: rows, cols: int(cols)}
}

func buildM(rows [][]object.Value, cols int) object.Value {
	outer := make([]object.Value, len(rows))
	for i, r := range rows {
		outer[i] = object.NewList(r...)
	}
	fields := make([]object.Value, 2)
	fields[fRows] = object.NewList(outer...)
	fields[fCols] = object.Int(int64(cols))
	return object.InstanceValue(&object.Instance{Cls: matrixClass, Fields: fields})
}

func newM(rows [][]object.Value) object.Value {
	cols := 0
	if len(rows) > 0 {
		cols = len(rows[0])
	}
	return buildM(rows, cols)
}

func isMatrix(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(matrixClass)
}

func isVector(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(vectorClass)
}

func isNumeric(v object.Value) bool {
	switch v.Kind() {
	case object.KindInt, object.KindBig, object.KindFloat:
		return true
	}
	return false
}

func dimError() error { return &object.Exception{Cls: errDim, Message: "Dimension mismatch"} }

func notRegular() error { return &object.Exception{Cls: errRegular, Message: "Not Regular Matrix"} }

func notDefined(op string, a, b object.Value) error {
	return &object.Exception{Cls: errUndef, Message: fmt.Sprintf("Operation(%s) can't be defined: %s op %s", op, stdlib.KindName(a), stdlib.KindName(b))}
}

func notImplemented(op string, a, b object.Value) error {
	return &object.Exception{Cls: errUnimpl, Message: fmt.Sprintf("Sorry, Operation(%s) not implemented: %s op %s", op, stdlib.KindName(a), stdlib.KindName(b))}
}

// Element arithmetic goes through the script's own operators, so the
// elements keep Ruby's Integer and Float behaviour.

func op(ctx object.Context, a object.Value, name string, b object.Value) (object.Value, error) {
	return ctx.Call(a, name, b)
}

func add(ctx object.Context, a, b object.Value) (object.Value, error) { return op(ctx, a, "+", b) }
func sub(ctx object.Context, a, b object.Value) (object.Value, error) { return op(ctx, a, "-", b) }
func mul(ctx object.Context, a, b object.Value) (object.Value, error) { return op(ctx, a, "*", b) }
func div(ctx object.Context, a, b object.Value) (object.Value, error) { return op(ctx, a, "/", b) }

func isZero(ctx object.Context, v object.Value) (bool, error) {
	return ctx.Equal(v, object.Int(0))
}

// dot is the sum of the products of two equal-length lists.
func dot(ctx object.Context, a, b []object.Value) (object.Value, error) {
	sum := object.Int(0)
	for i := range a {
		p, err := mul(ctx, a[i], b[i])
		if err != nil {
			return object.Value{}, err
		}
		if sum, err = add(ctx, sum, p); err != nil {
			return object.Value{}, err
		}
	}
	return sum, nil
}

// rowsArg reads an Array of Arrays, checking the row lengths as
// Matrix.rows does.
func rowsArg(ctx object.Context, v object.Value) ([][]object.Value, error) {
	outer, err := stdlib.ToList(ctx, v)
	if err != nil {
		return nil, err
	}
	rows := make([][]object.Value, len(outer))
	for i, r := range outer {
		row, err := stdlib.ToList(ctx, r)
		if err != nil {
			return nil, err
		}
		if i > 0 && len(row) != len(rows[0]) {
			return nil, &object.Exception{Cls: errDim, Message: fmt.Sprintf("row size differs (%d should be %d)", len(row), len(rows[0]))}
		}
		rows[i] = append([]object.Value(nil), row...)
	}
	return rows, nil
}

func listArg(ctx object.Context, v object.Value) ([]object.Value, error) {
	list, err := stdlib.ToList(ctx, v)
	if err != nil {
		return nil, err
	}
	return append([]object.Value(nil), list...), nil
}

// each lists the elements a which selects, with their positions.
type at struct {
	v    object.Value
	i, j int
}

// each lists the elements a which selects, with their positions; which
// is one whichArg read.
func (m matrix) each(which string) []at {
	var out []at
	for i, row := range m.rows {
		for j, v := range row {
			if m.selects(which, i, j) {
				out = append(out, at{v, i, j})
			}
		}
	}
	return out
}

// selectors are the names of each's which argument.
var selectors = []string{"all", "diagonal", "off_diagonal", "lower", "strict_lower", "strict_upper", "upper"}

// whichArg reads the which argument of each and kin: one of the
// selectors as a Symbol, :all when absent, and Ruby's ArgumentError for
// anything else, before any element is looked at.
func whichArg(ctx object.Context, args []object.Value) (string, error) {
	if len(args) == 0 {
		return "all", nil
	}
	if name, ok := args[0].AsSymbol(); ok && slices.Contains(selectors, name) {
		return name, nil
	}
	shown, err := ctx.Inspect(args[0])
	if err != nil {
		return "", err
	}
	return "", ctx.Raise(object.ArgumentError, "expected %s to be one of :all, :diagonal, :off_diagonal, :lower, :strict_lower, :strict_upper or :upper", shown)
}

// determinant is Matrix#determinant: the closed forms up to 3, Bareiss
// above, with the elements' own arithmetic.
func (m matrix) determinant(ctx object.Context) (object.Value, error) {
	if !m.square() {
		return object.Value{}, dimError()
	}
	a := m.rows
	switch len(a) {
	case 0:
		return object.Int(1), nil
	case 1:
		return a[0][0], nil
	case 2:
		p1, err := mul(ctx, a[0][0], a[1][1])
		if err != nil {
			return object.Value{}, err
		}
		p2, err := mul(ctx, a[0][1], a[1][0])
		if err != nil {
			return object.Value{}, err
		}
		return sub(ctx, p1, p2)
	case 3:
		// m0 * (m4*m8 - m5*m7) - m1 * (m3*m8 - m5*m6) + m2 * (m3*m7 - m4*m6),
		// in Ruby's order of operations.
		m0, m1, m2 := a[0][0], a[0][1], a[0][2]
		m3, m4, m5 := a[1][0], a[1][1], a[1][2]
		m6, m7, m8 := a[2][0], a[2][1], a[2][2]
		term := func(x, p, q, r, s object.Value) (object.Value, error) {
			pq, err := mul(ctx, p, q)
			if err != nil {
				return object.Value{}, err
			}
			rs, err := mul(ctx, r, s)
			if err != nil {
				return object.Value{}, err
			}
			d, err := sub(ctx, pq, rs)
			if err != nil {
				return object.Value{}, err
			}
			return mul(ctx, x, d)
		}
		t0, err := term(m0, m4, m8, m5, m7)
		if err != nil {
			return object.Value{}, err
		}
		t1, err := term(m1, m3, m8, m5, m6)
		if err != nil {
			return object.Value{}, err
		}
		t2, err := term(m2, m3, m7, m4, m6)
		if err != nil {
			return object.Value{}, err
		}
		d, err := sub(ctx, t0, t1)
		if err != nil {
			return object.Value{}, err
		}
		return add(ctx, d, t2)
	}
	return m.bareiss(ctx)
}

func (m matrix) bareiss(ctx object.Context) (object.Value, error) {
	size := len(m.rows)
	a := make([][]object.Value, size)
	for i, row := range m.rows {
		a[i] = append([]object.Value(nil), row...)
	}
	sign := 1
	pivot := object.Value(object.Int(1))
	for k := 0; k < size; k++ {
		previous := pivot
		pivot = a[k][k]
		if zero, err := isZero(ctx, pivot); err != nil {
			return object.Value{}, err
		} else if zero {
			swap := -1
			for row := k + 1; row < size; row++ {
				z, err := isZero(ctx, a[row][k])
				if err != nil {
					return object.Value{}, err
				}
				if !z {
					swap = row
					break
				}
			}
			if swap < 0 {
				return object.Int(0), nil
			}
			a[swap], a[k] = a[k], a[swap]
			pivot = a[k][k]
			sign = -sign
		}
		for i := k + 1; i < size; i++ {
			for j := k + 1; j < size; j++ {
				p, err := mul(ctx, pivot, a[i][j])
				if err != nil {
					return object.Value{}, err
				}
				q, err := mul(ctx, a[i][k], a[k][j])
				if err != nil {
					return object.Value{}, err
				}
				d, err := sub(ctx, p, q)
				if err != nil {
					return object.Value{}, err
				}
				if a[i][j], err = div(ctx, d, previous); err != nil {
					return object.Value{}, err
				}
			}
		}
	}
	if sign < 0 {
		return mul(ctx, object.Int(-1), pivot)
	}
	return pivot, nil
}

// rank is Matrix#rank, fraction-free elimination with the elements' own
// arithmetic.
func (m matrix) rank(ctx object.Context) (int, error) {
	a := make([][]object.Value, len(m.rows))
	for i, row := range m.rows {
		a[i] = append([]object.Value(nil), row...)
	}
	lastRow := len(a) - 1
	pivotRow := 0
	previous := object.Value(object.Int(1))
	for k := 0; k < m.cols; k++ {
		switchRow := -1
		for row := pivotRow; row <= lastRow; row++ {
			z, err := isZero(ctx, a[row][k])
			if err != nil {
				return 0, err
			}
			if !z {
				switchRow = row
				break
			}
		}
		if switchRow < 0 {
			continue
		}
		if switchRow != pivotRow {
			a[switchRow], a[pivotRow] = a[pivotRow], a[switchRow]
		}
		pivot := a[pivotRow][k]
		for i := pivotRow + 1; i <= lastRow; i++ {
			for j := k + 1; j < m.cols; j++ {
				p, err := mul(ctx, pivot, a[i][j])
				if err != nil {
					return 0, err
				}
				q, err := mul(ctx, a[i][k], a[pivotRow][j])
				if err != nil {
					return 0, err
				}
				d, err := sub(ctx, p, q)
				if err != nil {
					return 0, err
				}
				if a[i][j], err = div(ctx, d, previous); err != nil {
					return 0, err
				}
			}
		}
		pivotRow++
		previous = pivot
	}
	return pivotRow, nil
}

// inverse is Matrix#inverse, exactly for Integers, as Floats.
func (m matrix) inverse(ctx object.Context) (object.Value, error) {
	if !m.square() {
		return object.Value{}, dimError()
	}
	rows, err := inverseRows(m.rows)
	switch err.(type) {
	case nil:
	case errNotRegular:
		return object.Value{}, notRegular()
	default:
		return object.Value{}, ctx.Raise(object.TypeError, "%s", err.Error())
	}
	return buildM(rows, m.cols), nil
}

// times is Matrix * Matrix.
func (m matrix) times(ctx object.Context, o matrix) (object.Value, error) {
	if m.cols != len(o.rows) {
		return object.Value{}, dimError()
	}
	rows := make([][]object.Value, len(m.rows))
	for i, row := range m.rows {
		rows[i] = make([]object.Value, o.cols)
		for j := 0; j < o.cols; j++ {
			col := make([]object.Value, len(o.rows))
			for k := range o.rows {
				col[k] = o.rows[k][j]
			}
			v, err := dot(ctx, row, col)
			if err != nil {
				return object.Value{}, err
			}
			rows[i][j] = v
		}
	}
	return buildM(rows, o.cols), nil
}

// mapped applies f to every element.
func (m matrix) mapped(f func(v object.Value, i, j int) (object.Value, error)) (object.Value, error) {
	rows := make([][]object.Value, len(m.rows))
	for i, row := range m.rows {
		rows[i] = make([]object.Value, len(row))
		for j, v := range row {
			nv, err := f(v, i, j)
			if err != nil {
				return object.Value{}, err
			}
			rows[i][j] = nv
		}
	}
	return buildM(rows, m.cols), nil
}

func (m matrix) transpose() object.Value {
	if len(m.rows) == 0 {
		return buildM(make([][]object.Value, m.cols), 0)
	}
	rows := make([][]object.Value, m.cols)
	for j := 0; j < m.cols; j++ {
		rows[j] = make([]object.Value, len(m.rows))
		for i := range m.rows {
			rows[j][i] = m.rows[i][j]
		}
	}
	return buildM(rows, len(m.rows))
}

func identity(n int) object.Value {
	rows := make([][]object.Value, n)
	for i := range rows {
		rows[i] = make([]object.Value, n)
		for j := range rows[i] {
			rows[i][j] = object.Int(0)
		}
		rows[i][i] = object.Int(1)
	}
	return buildM(rows, n)
}

// power is Matrix ** Integer.
func (m matrix) power(ctx object.Context, self object.Value, exp int64) (object.Value, error) {
	if !m.square() {
		return object.Value{}, dimError()
	}
	if exp == 0 {
		return identity(m.cols), nil
	}
	x := self
	if exp < 0 {
		inv, err := m.inverse(ctx)
		if err != nil {
			return object.Value{}, err
		}
		x, exp = inv, -exp
	}
	var result object.Value
	for exp > 0 {
		if exp&1 == 1 {
			if result.IsZero() {
				result = x
			} else {
				var err error
				if result, err = readM(result).times(ctx, readM(x)); err != nil {
					return object.Value{}, err
				}
			}
		}
		exp >>= 1
		if exp > 0 {
			var err error
			if x, err = readM(x).times(ctx, readM(x)); err != nil {
				return object.Value{}, err
			}
		}
	}
	return result, nil
}

// minorOf is Matrix#minor with Ruby's Array slicing at the edges.
func (m matrix) minorOf(fromRow, sizeRow, fromCol, sizeCol int) object.Value {
	if sizeRow < 0 || sizeCol < 0 {
		return object.Nil
	}
	if fromRow < 0 {
		fromRow += len(m.rows)
	}
	if fromCol < 0 {
		fromCol += m.cols
	}
	if fromRow > len(m.rows) || fromCol > m.cols || fromRow < 0 || fromCol < 0 {
		return object.Nil
	}
	// The sizes are clamped before they are added, so a huge one does not
	// overflow.
	toRow := fromRow + min(sizeRow, len(m.rows)-fromRow)
	toCol := fromCol + min(sizeCol, m.cols-fromCol)
	rows := make([][]object.Value, 0, toRow-fromRow)
	for i := fromRow; i < toRow; i++ {
		rows = append(rows, append([]object.Value(nil), m.rows[i][fromCol:toCol]...))
	}
	return buildM(rows, min(m.cols-fromCol, sizeCol))
}

// firstMinor drops a row and a column.
func (m matrix) firstMinor(row, col int) object.Value {
	rows := make([][]object.Value, 0, len(m.rows)-1)
	for i, r := range m.rows {
		if i == row {
			continue
		}
		nr := make([]object.Value, 0, m.cols-1)
		for j, v := range r {
			if j != col {
				nr = append(nr, v)
			}
		}
		rows = append(rows, nr)
	}
	return buildM(rows, m.cols-1)
}

func (m matrix) cofactor(ctx object.Context, row, col int) (object.Value, error) {
	if len(m.rows) == 0 || !m.square() {
		return object.Value{}, dimError()
	}
	d, err := readM(m.firstMinor(row, col)).determinant(ctx)
	if err != nil {
		return object.Value{}, err
	}
	if (row+col)%2 == 1 {
		return mul(ctx, d, object.Int(-1))
	}
	return mul(ctx, d, object.Int(1))
}

// render is to_s and inspect: Matrix[[1, 2], [3, 4]] or Matrix.empty(r, c).
func (m matrix) render(ctx object.Context, inspect bool) (string, error) {
	if len(m.rows) == 0 || m.cols == 0 {
		return fmt.Sprintf("Matrix.empty(%d, %d)", len(m.rows), m.cols), nil
	}
	var b strings.Builder
	b.WriteString("Matrix[")
	for i, row := range m.rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("[")
		for j, v := range row {
			if j > 0 {
				b.WriteString(", ")
			}
			s, err := renderElem(ctx, v, inspect)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		}
		b.WriteString("]")
	}
	b.WriteString("]")
	return b.String(), nil
}

func renderElem(ctx object.Context, v object.Value, inspect bool) (string, error) {
	if inspect {
		return ctx.Inspect(v)
	}
	return ctx.ToS(v)
}

// Install defines Matrix, Vector and their errors. lib.Install calls it
// once.
func Install() {
	object.DefineNative("Matrix", object.ClassValue(matrixClass))
	object.DefineNative("Vector", object.ClassValue(vectorClass))
	object.DefineNative("ExceptionForMatrix", object.ClassValue(errorsMod))
	for _, e := range []*object.Class{errDim, errRegular, errUndef, errUnimpl} {
		e.Lexical = errorsMod
		errorsMod.Consts[e.Name] = object.ClassValue(e)
		matrixClass.Consts[e.Name] = object.ClassValue(e)
		vectorClass.Consts[e.Name] = object.ClassValue(e)
	}
	errZeroVec.Lexical = vectorClass
	vectorClass.Consts["ZeroVectorError"] = object.ClassValue(errZeroVec)
	installScalar()
	used := map[int64]bool{}
	taken := func(number int64) bool {
		if used[number] {
			return true
		}
		used[number] = true
		return false
	}
	elems := &object.FieldType{Kind: "list", Elem: object.ScalarField("float")}
	declare := func(cls *object.Class, name string, typ *object.FieldType) {
		f := &object.Field{Name: name, Number: protogen.FieldNumber(name, taken), Type: typ, Owner: cls}
		cls.FieldIndex[name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
	declare(matrixClass, "rows", &object.FieldType{Kind: "list", Elem: elems})
	declare(matrixClass, "column_count", object.ScalarField("int"))
	declare(vectorClass, "elements", elems)
	matrixClass.Include(object.Enumerable)
	vectorClass.Include(object.Enumerable)
	installMatrixClass()
	installMatrix()
	installVector()
}

type matrixFn func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

func defM(name string, arity int, f matrixFn) {
	matrixClass.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return f(ctx, readM(self), self, args, kw, block)
	})
}

func defS(name string, arity int, f object.NativeFn) { matrixClass.DefineSingleton(name, arity, f) }

func intArg(ctx object.Context, v object.Value) (int, error) {
	n, err := stdlib.WantInt(ctx, v, "")
	return int(n), err
}

// sizeArg reads a size, which Ruby's Array.new refuses below zero.
func sizeArg(ctx object.Context, v object.Value) (int, error) {
	n, err := intArg(ctx, v)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, ctx.Raise(object.ArgumentError, "negative array size")
	}
	return n, nil
}

// installMatrixClass defines the constructors.
func installMatrixClass() {
	defS("[]", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		rows, err := rowsArg(ctx, object.NewList(args...))
		if err != nil {
			return object.Value{}, err
		}
		return newM(rows), nil
	})
	defS("rows", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		rows, err := rowsArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return newM(rows), nil
	})
	defS("columns", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		cols, err := rowsArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return readM(newM(cols)).transpose(), nil
	})
	defS("build", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		r, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		c := r
		if len(args) == 2 {
			if c, err = intArg(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		if r < 0 || c < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "Negative size")
		}
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.ArgumentError, "Matrix.build needs a block")
		}
		rows := make([][]object.Value, r)
		for i := range rows {
			rows[i] = make([]object.Value, c)
			for j := range rows[i] {
				v, err := ctx.Yield(block, object.Int(int64(i)), object.Int(int64(j)))
				if err != nil {
					return object.Value{}, err
				}
				rows[i][j] = v
			}
		}
		return buildM(rows, c), nil
	})
	diagonal := func(values []object.Value) object.Value {
		n := len(values)
		rows := make([][]object.Value, n)
		for i := range rows {
			rows[i] = make([]object.Value, n)
			for j := range rows[i] {
				rows[i][j] = object.Int(0)
			}
			rows[i][i] = values[i]
		}
		return buildM(rows, n)
	}
	defS("diagonal", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return diagonal(args), nil
	})
	defS("scalar", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := sizeArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		values := make([]object.Value, n)
		for i := range values {
			values[i] = args[1]
		}
		return diagonal(values), nil
	})
	for _, name := range []string{"identity", "unit", "I"} {
		defS(name, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			n, err := sizeArg(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return identity(n), nil
		})
	}
	defS("zero", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		r, err := sizeArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		c := r
		if len(args) == 2 {
			if c, err = sizeArg(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		rows := make([][]object.Value, r)
		for i := range rows {
			rows[i] = make([]object.Value, c)
			for j := range rows[i] {
				rows[i][j] = object.Int(0)
			}
		}
		return buildM(rows, c), nil
	})
	defS("row_vector", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		row, err := listArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return buildM([][]object.Value{row}, len(row)), nil
	})
	defS("column_vector", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		col, err := listArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		rows := make([][]object.Value, len(col))
		for i, v := range col {
			rows[i] = []object.Value{v}
		}
		return buildM(rows, 1), nil
	})
	defS("empty", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		r, c := 0, 0
		var err error
		if len(args) > 0 {
			if r, err = intArg(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		if len(args) > 1 {
			if c, err = intArg(ctx, args[1]); err != nil {
				return object.Value{}, err
			}
		}
		if r != 0 && c != 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "One size must be 0")
		}
		if r < 0 || c < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "Negative size")
		}
		rows := make([][]object.Value, r)
		for i := range rows {
			rows[i] = []object.Value{}
		}
		return buildM(rows, c), nil
	})
	stack := func(vertical bool) object.NativeFn {
		return func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 1 {
				return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
			}
			for _, a := range args {
				if !isMatrix(a) {
					return object.Value{}, ctx.Raise(object.TypeError, "Expected a Matrix, got a %s", stdlib.KindName(a))
				}
			}
			first := readM(args[0])
			if vertical {
				var rows [][]object.Value
				for _, a := range args {
					m := readM(a)
					if m.cols != first.cols {
						return object.Value{}, &object.Exception{Cls: errDim, Message: fmt.Sprintf("The given matrices must have %d columns, but one has %d", first.cols, m.cols)}
					}
					for _, r := range m.rows {
						rows = append(rows, append([]object.Value(nil), r...))
					}
				}
				return buildM(rows, first.cols), nil
			}
			rows := make([][]object.Value, len(first.rows))
			cols := 0
			for _, a := range args {
				m := readM(a)
				if len(m.rows) != len(first.rows) {
					return object.Value{}, &object.Exception{Cls: errDim, Message: fmt.Sprintf("The given matrices must have %d rows, but one has %d", len(first.rows), len(m.rows))}
				}
				for i, r := range m.rows {
					rows[i] = append(rows[i], r...)
				}
				cols += m.cols
			}
			return buildM(rows, cols), nil
		}
	}
	defS("vstack", -1, stack(true))
	defS("hstack", -1, stack(false))
	defS("combine", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combine(ctx, args, block)
	})
}

// combine is Matrix.combine: the block gets the elements at each place.
func combine(ctx object.Context, args []object.Value, block object.Value) (object.Value, error) {
	if len(args) == 0 {
		return buildM(nil, 0), nil
	}
	ms := make([]matrix, len(args))
	for i, a := range args {
		if !isMatrix(a) {
			return object.Value{}, ctx.Raise(object.TypeError, "Expected a Matrix, got a %s", stdlib.KindName(a))
		}
		ms[i] = readM(a)
		if len(ms[i].rows) != len(ms[0].rows) || ms[i].cols != ms[0].cols {
			return object.Value{}, dimError()
		}
	}
	return ms[0].mapped(func(v object.Value, i, j int) (object.Value, error) {
		at := make([]object.Value, len(ms))
		for k, m := range ms {
			at[k] = m.rows[i][j]
		}
		return ctx.Yield(block, at...)
	})
}

// installMatrix defines the instance methods.
func installMatrix() {
	defM("row_count", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(m.rows))), nil
	})
	defM("row_size", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(m.rows))), nil
	})
	defM("column_count", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(m.cols)), nil
	})
	defM("column_size", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(m.cols)), nil
	})
	defM("square?", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(m.square()), nil
	})
	defM("empty?", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(len(m.rows) == 0 || m.cols == 0), nil
	})
	element := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		j, err := intArg(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 {
			i += len(m.rows)
		}
		if j < 0 {
			j += m.cols
		}
		if i < 0 || i >= len(m.rows) || j < 0 || j >= m.cols {
			return object.Nil, nil
		}
		return m.rows[i][j], nil
	}
	defM("[]", 2, element)
	defM("element", 2, element)
	defM("component", 2, element)
	defM("row", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if i < 0 {
			i += len(m.rows)
		}
		if i < 0 || i >= len(m.rows) {
			return object.Nil, nil
		}
		if !block.IsZero() {
			for _, v := range m.rows[i] {
				if _, err := ctx.Yield(block, v); err != nil {
					return object.Value{}, err
				}
			}
			return self, nil
		}
		return newV(append([]object.Value(nil), m.rows[i]...)), nil
	})
	defM("column", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		j, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		if j < 0 {
			j += m.cols
		}
		if j < 0 || j >= m.cols {
			return object.Nil, nil
		}
		col := make([]object.Value, len(m.rows))
		for i := range m.rows {
			col[i] = m.rows[i][j]
		}
		if !block.IsZero() {
			for _, v := range col {
				if _, err := ctx.Yield(block, v); err != nil {
					return object.Value{}, err
				}
			}
			return self, nil
		}
		return newV(col), nil
	})
	defM("row_vectors", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, len(m.rows))
		for i, r := range m.rows {
			out[i] = newV(append([]object.Value(nil), r...))
		}
		return object.NewList(out...), nil
	})
	defM("column_vectors", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out := make([]object.Value, m.cols)
		for j := 0; j < m.cols; j++ {
			col := make([]object.Value, len(m.rows))
			for i := range m.rows {
				col[i] = m.rows[i][j]
			}
			out[j] = newV(col)
		}
		return object.NewList(out...), nil
	})
	defM("to_a", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self.Instance().Fields[fRows], nil
	})
	defM("to_matrix", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	// Iteration.
	defM("each", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		which, err := whichArg(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		items := m.each(which)
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, it := range items {
					if err := yield(it.v); err != nil {
						return err
					}
				}
				return nil
			}, Size: func(ctx object.Context) (object.Value, error) { return object.Int(int64(len(items))), nil }}), nil
		}
		for _, it := range items {
			if _, err := ctx.Yield(block, it.v); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	defM("each_with_index", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		which, err := whichArg(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		items := m.each(which)
		triple := func(it at) object.Value {
			return object.NewList(it.v, object.Int(int64(it.i)), object.Int(int64(it.j)))
		}
		if block.IsZero() {
			return object.EnumeratorValue(&object.Enumerator{Recv: self, Meth: "each_with_index", Each: func(ctx object.Context, yield func(object.Value) error) error {
				for _, it := range items {
					if err := yield(triple(it)); err != nil {
						return err
					}
				}
				return nil
			}, Size: func(ctx object.Context) (object.Value, error) { return object.Int(int64(len(items))), nil }}), nil
		}
		for _, it := range items {
			if _, err := ctx.Yield(block, it.v, object.Int(int64(it.i)), object.Int(int64(it.j))); err != nil {
				return object.Value{}, err
			}
		}
		return self, nil
	})
	collect := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		which, err := whichArg(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.ArgumentError, "collect needs a block")
		}
		return m.mapped(func(v object.Value, i, j int) (object.Value, error) {
			if !m.selects(which, i, j) {
				return v, nil
			}
			return ctx.Yield(block, v)
		})
	}
	defM("collect", -1, collect)
	defM("map", -1, collect)
	findIndex := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		which := "all"
		var want object.Value
		for _, a := range args {
			if a.Kind() == object.KindSymbol {
				var err error
				if which, err = whichArg(ctx, []object.Value{a}); err != nil {
					return object.Value{}, err
				}
			} else {
				want = a
			}
		}
		items := m.each(which)
		for _, it := range items {
			var hit bool
			var err error
			if !want.IsZero() {
				if hit, err = ctx.Equal(it.v, want); err != nil {
					return object.Value{}, err
				}
			} else if !block.IsZero() {
				r, err := ctx.Yield(block, it.v)
				if err != nil {
					return object.Value{}, err
				}
				hit = r.Truthy()
			}
			if hit {
				return object.NewList(object.Int(int64(it.i)), object.Int(int64(it.j))), nil
			}
		}
		return object.Nil, nil
	}
	defM("find_index", -1, findIndex)
	defM("index", -1, findIndex)
	// Arithmetic.
	defM("+", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return addSub(ctx, m, self, args[0], "+", add)
	})
	defM("-", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return addSub(ctx, m, self, args[0], "-", sub)
	})
	defM("*", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o := args[0]
		switch {
		case isNumeric(o):
			return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return mul(ctx, v, o) })
		case isVector(o):
			elems := readV(o)
			if m.cols != len(elems) {
				return object.Value{}, dimError()
			}
			out := make([]object.Value, len(m.rows))
			for i, row := range m.rows {
				v, err := dot(ctx, row, elems)
				if err != nil {
					return object.Value{}, err
				}
				out[i] = v
			}
			return newV(out), nil
		case isMatrix(o):
			return m.times(ctx, readM(o))
		}
		return object.Value{}, notDefined("*", self, o)
	})
	defM("/", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		o := args[0]
		switch {
		case isNumeric(o):
			return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return div(ctx, v, o) })
		case isMatrix(o):
			inv, err := readM(o).inverse(ctx)
			if err != nil {
				return object.Value{}, err
			}
			return m.times(ctx, readM(inv))
		}
		return object.Value{}, notDefined("/", self, o)
	})
	defM("**", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if n, ok := args[0].AsInt(); ok {
			return m.power(ctx, self, n)
		}
		if isNumeric(args[0]) {
			return object.Value{}, notImplemented("**", self, args[0])
		}
		return object.Value{}, notDefined("**", self, args[0])
	})
	defM("-@", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return ctx.Call(v, "-@") })
	})
	defM("+@", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	})
	hadamard := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combine(ctx, []object.Value{self, args[0]}, ctx.Block(func(xs []object.Value) (object.Value, error) {
			return mul(ctx, xs[0], xs[1])
		}))
	}
	defM("hadamard_product", 1, hadamard)
	defM("entrywise_product", 1, hadamard)
	defM("combine", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return combine(ctx, append([]object.Value{self}, args...), block)
	})
	// Properties.
	for _, name := range []string{"determinant", "det"} {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return m.determinant(ctx)
		})
	}
	for _, name := range []string{"inverse", "inv"} {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return m.inverse(ctx)
		})
	}
	for _, name := range []string{"transpose", "t"} {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return m.transpose(), nil
		})
	}
	defM("rank", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		r, err := m.rank(ctx)
		return object.Int(int64(r)), err
	})
	for _, name := range []string{"trace", "tr"} {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if !m.square() {
				return object.Value{}, dimError()
			}
			sum := object.Value(object.Int(0))
			for i := range m.rows {
				var err error
				if sum, err = add(ctx, sum, m.rows[i][i]); err != nil {
					return object.Value{}, err
				}
			}
			return sum, nil
		})
	}
	defM("minor", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		switch len(args) {
		case 2:
			fromRow, sizeRow, err := rangeArg(ctx, args[0], len(m.rows))
			if err != nil {
				return object.Value{}, err
			}
			fromCol, sizeCol, err := rangeArg(ctx, args[1], m.cols)
			if err != nil {
				return object.Value{}, err
			}
			return m.minorOf(fromRow, sizeRow, fromCol, sizeCol), nil
		case 4:
			ns := make([]int, 4)
			for i, a := range args {
				var err error
				if ns[i], err = intArg(ctx, a); err != nil {
					return object.Value{}, err
				}
			}
			return m.minorOf(ns[0], ns[1], ns[2], ns[3]), nil
		}
		return object.Value{}, stdlib.ArgError(ctx, len(args), "2 or 4")
	})
	defM("first_minor", 2, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		row, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		col, err := intArg(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		if len(m.rows) == 0 || m.cols == 0 {
			return object.Value{}, ctx.Raise(object.RuntimeError, "first_minor of empty matrix is not defined")
		}
		if row < 0 || row >= len(m.rows) {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid row (%d for 0..%d)", row, len(m.rows)-1)
		}
		if col < 0 || col >= m.cols {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid column (%d for 0..%d)", col, m.cols-1)
		}
		return m.firstMinor(row, col), nil
	})
	defM("cofactor", 2, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		row, err := intArg(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		col, err := intArg(ctx, args[1])
		if err != nil {
			return object.Value{}, err
		}
		if len(m.rows) == 0 || m.cols == 0 {
			return object.Value{}, ctx.Raise(object.RuntimeError, "cofactor of empty matrix is not defined")
		}
		return m.cofactor(ctx, row, col)
	})
	defM("adjugate", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !m.square() {
			return object.Value{}, dimError()
		}
		return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return m.cofactor(ctx, j, i) })
	})
	laplace := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var rowArg, colArg object.Value
		if kw != nil {
			rowArg, _ = kw.Get(object.Symbol("row"))
			colArg, _ = kw.Get(object.Symbol("column"))
		}
		hasRow, hasCol := !rowArg.IsZero() && !rowArg.IsNil(), !colArg.IsZero() && !colArg.IsNil()
		if hasRow == hasCol {
			return object.Value{}, ctx.Raise(object.ArgumentError, "exactly one the row or column arguments must be specified")
		}
		if !m.square() {
			return object.Value{}, dimError()
		}
		if len(m.rows) == 0 {
			return object.Value{}, ctx.Raise(object.RuntimeError, "laplace_expansion of empty matrix is not defined")
		}
		if len(m.rows) == 1 {
			return m.rows[0][0], nil
		}
		num, err := intArg(ctx, rowArg)
		if hasCol {
			num, err = intArg(ctx, colArg)
		}
		if err != nil {
			return object.Value{}, err
		}
		if num < 0 || num >= len(m.rows) {
			return object.Value{}, ctx.Raise(object.ArgumentError, "invalid num (%d for 0..%d)", num, len(m.rows)-1)
		}
		sum := object.Value(object.Int(0))
		for i := 0; i < len(m.rows); i++ {
			var e, c object.Value
			if hasRow {
				e = m.rows[num][i]
				c, err = m.cofactor(ctx, num, i)
			} else {
				e = m.rows[i][num]
				c, err = m.cofactor(ctx, i, num)
			}
			if err != nil {
				return object.Value{}, err
			}
			p, err := mul(ctx, e, c)
			if err != nil {
				return object.Value{}, err
			}
			if sum, err = add(ctx, sum, p); err != nil {
				return object.Value{}, err
			}
		}
		return sum, nil
	}
	defM("laplace_expansion", -1, laplace)
	defM("cofactor_expansion", -1, laplace)
	// Predicates.
	pred := func(name string, needSquare bool, test func(ctx object.Context, m matrix) (bool, error)) {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if needSquare && !m.square() {
				return object.Value{}, dimError()
			}
			ok, err := test(ctx, m)
			return object.Bool(ok), err
		})
	}
	allWhere := func(ctx object.Context, m matrix, keep func(i, j int) bool, want func(i, j int) (object.Value, error)) (bool, error) {
		for i, row := range m.rows {
			for j, v := range row {
				if !keep(i, j) {
					continue
				}
				w, err := want(i, j)
				if err != nil {
					return false, err
				}
				eq, err := ctx.Equal(v, w)
				if err != nil || !eq {
					return false, err
				}
			}
		}
		return true, nil
	}
	zero := func(i, j int) (object.Value, error) { return object.Int(0), nil }
	pred("symmetric?", true, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return i != j }, func(i, j int) (object.Value, error) { return m.rows[j][i], nil })
	})
	anti := func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return true }, func(i, j int) (object.Value, error) {
			return ctx.Call(m.rows[j][i], "-@")
		})
	}
	pred("antisymmetric?", true, anti)
	pred("skew_symmetric?", true, anti)
	pred("diagonal?", true, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return i != j }, zero)
	})
	pred("lower_triangular?", false, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return i < j }, zero)
	})
	pred("upper_triangular?", false, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return i > j }, zero)
	})
	pred("zero?", false, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return true }, zero)
	})
	pred("real?", false, func(ctx object.Context, m matrix) (bool, error) { return true, nil })
	pred("regular?", true, func(ctx object.Context, m matrix) (bool, error) {
		d, err := m.determinant(ctx)
		if err != nil {
			return false, err
		}
		z, err := isZero(ctx, d)
		return !z, err
	})
	pred("singular?", true, func(ctx object.Context, m matrix) (bool, error) {
		d, err := m.determinant(ctx)
		if err != nil {
			return false, err
		}
		return isZero(ctx, d)
	})
	orthogonal := func(ctx object.Context, m matrix) (bool, error) {
		for i, ri := range m.rows {
			for j, rj := range m.rows {
				s, err := dot(ctx, ri, rj)
				if err != nil {
					return false, err
				}
				want := object.Int(0)
				if i == j {
					want = object.Int(1)
				}
				eq, err := ctx.Equal(s, want)
				if err != nil || !eq {
					return false, err
				}
			}
		}
		return true, nil
	}
	pred("orthogonal?", true, orthogonal)
	pred("unitary?", true, orthogonal)
	pred("normal?", true, func(ctx object.Context, m matrix) (bool, error) {
		t := readM(m.transpose())
		a, err := m.times(ctx, t)
		if err != nil {
			return false, err
		}
		b, err := t.times(ctx, m)
		if err != nil {
			return false, err
		}
		return ctx.Equal(a, b)
	})
	pred("hermitian?", true, func(ctx object.Context, m matrix) (bool, error) {
		return allWhere(ctx, m, func(i, j int) bool { return i != j }, func(i, j int) (object.Value, error) { return m.rows[j][i], nil })
	})
	pred("permutation?", true, func(ctx object.Context, m matrix) (bool, error) {
		seenCol := make([]bool, m.cols)
		for _, row := range m.rows {
			found := false
			for j, v := range row {
				one, err := ctx.Equal(v, object.Int(1))
				if err != nil {
					return false, err
				}
				z, err := isZero(ctx, v)
				if err != nil {
					return false, err
				}
				switch {
				case one && !found && !seenCol[j]:
					found, seenCol[j] = true, true
				case z:
				default:
					return false, nil
				}
			}
			if !found {
				return false, nil
			}
		}
		return true, nil
	})
	// Comparison and rendering.
	defM("==", 1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !isMatrix(args[0]) {
			return object.False, nil
		}
		o := readM(args[0])
		if m.cols != o.cols {
			return object.False, nil
		}
		eq, err := ctx.Equal(self.Instance().Fields[fRows], args[0].Instance().Fields[fRows])
		return object.Bool(eq), err
	})
	defM("to_s", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := m.render(ctx, false)
		return object.NewString(s, str.UTF8), err
	})
	defM("inspect", 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := m.render(ctx, true)
		return object.NewString(s, str.UTF8), err
	})
	// Shapes and copies.
	defM("vstack", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Call(object.ClassValue(matrixClass), "vstack", append([]object.Value{self}, args...)...)
	})
	defM("hstack", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return ctx.Call(object.ClassValue(matrixClass), "hstack", append([]object.Value{self}, args...)...)
	})
	defM("rotate_entries", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		turns := 1
		if len(args) > 0 {
			switch {
			case args[0].Kind() == object.KindSymbol:
				switch args[0].Str() {
				case "clockwise":
					turns = 1
				case "counter_clockwise":
					turns = 3
				case "half_turn":
					turns = 2
				default:
					shown, _ := ctx.Inspect(args[0])
					return object.Value{}, ctx.Raise(object.ArgumentError, "expected %s to be one of :clockwise, :counter_clockwise, :half_turn or an integer", shown)
				}
			default:
				n, err := intArg(ctx, args[0])
				if err != nil {
					return object.Value{}, err
				}
				turns = ((n % 4) + 4) % 4
			}
		}
		out := m
		for ; turns > 0; turns-- {
			// A clockwise turn: the rows of the transpose, each reversed.
			t := readM(out.transpose())
			for _, row := range t.rows {
				for a, b := 0, len(row)-1; a < b; a, b = a+1, b-1 {
					row[a], row[b] = row[b], row[a]
				}
			}
			out = t
		}
		return buildM(out.rows, out.cols), nil
	})
	defM("round", -1, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return ctx.Call(v, "round", args...) })
	})
	same := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return self, nil
	}
	for _, name := range []string{"conj", "conjugate", "real"} {
		defM(name, 0, same)
	}
	zeroM := func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return object.Int(0), nil })
	}
	defM("imag", 0, zeroM)
	defM("imaginary", 0, zeroM)
	for _, name := range []string{"rect", "rectangular"} {
		defM(name, 0, func(ctx object.Context, m matrix, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			z, err := zeroM(ctx, m, self, args, kw, block)
			if err != nil {
				return object.Value{}, err
			}
			return object.NewList(self, z), nil
		})
	}
}

// addSub is Matrix + and -: with a Matrix of the same shape, with a
// Vector as a column, and not with a number.
func addSub(ctx object.Context, m matrix, self, o object.Value, name string, f func(object.Context, object.Value, object.Value) (object.Value, error)) (object.Value, error) {
	var other matrix
	switch {
	case isNumeric(o):
		return object.Value{}, notDefined(name, self, o)
	case isVector(o):
		elems := readV(o)
		rows := make([][]object.Value, len(elems))
		for i, v := range elems {
			rows[i] = []object.Value{v}
		}
		other = matrix{rows: rows, cols: 1}
	case isMatrix(o):
		other = readM(o)
	default:
		return object.Value{}, notDefined(name, self, o)
	}
	if len(m.rows) != len(other.rows) || m.cols != other.cols {
		return object.Value{}, dimError()
	}
	return m.mapped(func(v object.Value, i, j int) (object.Value, error) { return f(ctx, v, other.rows[i][j]) })
}

// selects says whether a which selects the place (i, j).
func (matrix) selects(which string, i, j int) bool {
	switch which {
	case "diagonal":
		return i == j
	case "off_diagonal":
		return i != j
	case "lower":
		return i >= j
	case "strict_lower":
		return i > j
	case "upper":
		return i <= j
	case "strict_upper":
		return i < j
	}
	return true
}

// rangeArg reads a Range as minor does: the start and the size, counting
// a negative bound from the end.
func rangeArg(ctx object.Context, v object.Value, count int) (from, size int, err error) {
	r, ok := v.AsRange()
	if !ok {
		return 0, 0, ctx.Raise(object.TypeError, "minor takes two Ranges or four Integers")
	}
	from, err = intArg(ctx, r.Lo)
	if err != nil {
		return 0, 0, err
	}
	if from < 0 {
		from += count
	}
	to := count - 1
	if !r.Hi.IsNil() {
		if to, err = intArg(ctx, r.Hi); err != nil {
			return 0, 0, err
		}
	}
	if to < 0 {
		to += count
	}
	if !r.Exclusive || r.Hi.IsNil() {
		to++
	}
	return from, to - from, nil
}
