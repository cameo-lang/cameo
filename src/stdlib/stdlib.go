// Package stdlib is amber's standard library: the methods of the builtin
// classes, implemented in Go against the object package. Everything returns
// a new value; nothing changes its receiver.
package stdlib

import (
	"fmt"

	"github.com/radiospiel/amber/src/object"
)

// Install defines every builtin method. The evaluator calls it once.
func Install() {
	installObject()
	installKernel()
	installNilBool()
	installNumeric()
	installString()
	installBytes()
	installSymbol()
	installException()
	installClass()
	installEnumerable()
	installList()
	installMap()
	installSet()
	installRange()
	installTime()
	installMath()
	installRegexp()
	installStringPatterns()
	installEnumerator()
	installTimeParsing()
	installCGI()
	installPaths()
}

type fn = func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

// def is a shorthand for defining a method with a fixed arity.
func def(c *object.Class, name string, arity int, f fn) { c.Define(name, arity, f) }

// argError makes the ArgumentError for a wrong count.
func argError(ctx object.Context, given int, expected string) error {
	return ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected %s)", given, expected)
}

// wantInt reads an Integer argument or fails with a TypeError.
func wantInt(ctx object.Context, v object.Value, what string) (int64, error) {
	switch v.Kind() {
	case object.KindInt:
		v := v.Int()
		return int64(v), nil
	case object.KindFloat:
		v := v.Float()
		return int64(v), nil
	case object.KindBig:
		return 0, ctx.Raise(object.RangeError, "bignum too big to convert into 'long'")
	}
	return 0, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer%s", kindName(v), what)
}

// wantString reads a String argument or fails with a TypeError.
func wantString(ctx object.Context, v object.Value) (string, error) {
	switch v.Kind() {
	case object.KindString:
		v := v.Str()
		return string(v), nil
	case object.KindSymbol:
		v := v.Str()
		return string(v), nil
	}
	return "", ctx.Raise(object.TypeError, "no implicit conversion of %s into String", kindName(v))
}

// kindName names a value's class in a TypeError, the way Ruby does.
func kindName(v object.Value) string {
	if v.IsNil() {
		return "nil"
	}
	switch v.Kind() {
	case object.KindBool:
		return fmt.Sprint(v.Truthy())
	}
	return v.Class().QualifiedName()
}

// blockOrEnumerator fails when a method that needs a block has none.
func needBlock(ctx object.Context, block object.Value, name string) error {
	if block.IsZero() {
		return ctx.Raise(object.LocalJumpError, "no block given (%s)", name)
	}
	return nil
}

// toList reads an iterable argument as a list of elements.
func toList(ctx object.Context, v object.Value) ([]object.Value, error) {
	switch v.Kind() {
	case object.KindList:
		v := v.List()
		return v.Elems, nil
	case object.KindSet:
		v := v.Set()
		return v.Elems(), nil
	case object.KindRange:
		v := v.Range()
		return RangeElems(ctx, v)
	case object.KindEnumerator:
		return enumElems(ctx, v.Enumerator())
	case object.KindMap:
		v := v.Map()
		out := make([]object.Value, v.Len())
		for i, k := range v.Keys() {
			out[i] = object.NewList(k, v.Values()[i])
		}
		return out, nil
	}
	return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into List", kindName(v))
}
