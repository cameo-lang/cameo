// Package stdlib is cameo's core: the methods of the builtin classes,
// implemented in Go against the object package. Everything returns a new
// value; nothing changes its receiver. The libraries a script requires are
// under src/lib, one package each; they take what they share with the
// core, the argument checks every builtin method makes and the pieces two
// libraries share, from this package's exported functions, and stdlib
// imports no library.
package stdlib

import (
	"fmt"

	"github.com/cameo-lang/cameo/src/object"
)

// Install defines every builtin method. The evaluator calls it once.
func Install() {
	installObject()
	installKernel()
	installRandom()
	installNilBool()
	installNumeric()
	installString()
	installStringBytes()
	installSymbol()
	installException()
	installClass()
	installEnumerable()
	installFolds()
	installList()
	installMap()
	installSet()
	installRange()
	installTime()
	installMath()
	installProcess()
	installRegexp()
	installStringPatterns()
	installEnumerator()
	installEncodings()
	installPack()
	installPaths()
	installVersion()
	installIO()
	// After Enumerable and Enumerator: it wraps Enumerable's methods.
	installUserEnumerable()
	// The methods read for it: each uses its arguments and returns.
	borrows(object.MapClass, "[]", "key?", "has_key?", "include?", "member?")
	borrows(object.ListClass, "[]", "at")
}

// borrows marks the methods names of c as ones that do not keep the
// slice of their arguments (object.Method.Borrows). A method is marked
// once its code was read for that: it may keep an argument, and pass
// the slice down, but neither return nor store the slice or a part of
// it.
func borrows(c *object.Class, names ...string) {
	for _, name := range names {
		m := c.Methods[name]
		if m == nil || m.Native == nil {
			panic("stdlib: no builtin " + c.Name + "#" + name)
		}
		m.Borrows = true
	}
}

type fn = func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)

// def is a shorthand for defining a method with a fixed arity.
func def(c *object.Class, name string, arity int, f fn) { c.Define(name, arity, f) }

// ArgError is the ArgumentError for a wrong number of arguments:
// "wrong number of arguments (given 2, expected 1..3)".
func ArgError(ctx object.Context, given int, expected string) error {
	return ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected %s)", given, expected)
}

// WantInt reads an Integer argument, a Float truncated, or fails: a
// TypeError for what is no number, a RangeError for an Integer beyond 64
// bits; what names the argument after "into Integer" in the message.
func WantInt(ctx object.Context, v object.Value, what string) (int64, error) {
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
	return 0, ctx.Raise(object.TypeError, "no implicit conversion of %s into Integer%s", KindName(v), what)
}

// WantString reads a String or Symbol argument or fails with Ruby's
// TypeError.
func WantString(ctx object.Context, v object.Value) (string, error) {
	switch v.Kind() {
	case object.KindString, object.KindBytes, object.KindASCII:
		v := v.Str()
		return string(v), nil
	case object.KindSymbol:
		v := v.Str()
		return string(v), nil
	}
	return "", ctx.Raise(object.TypeError, "no implicit conversion of %s into String", KindName(v))
}

// KindName names a value's class as a TypeError does: "nil", "true",
// "Integer".
func KindName(v object.Value) string {
	if v.IsNil() {
		return "nil"
	}
	switch v.Kind() {
	case object.KindBool:
		return fmt.Sprint(v.Truthy())
	}
	return v.Class().QualifiedName()
}

// NeedBlock fails with a LocalJumpError when a method that needs a block
// has none.
func NeedBlock(ctx object.Context, block object.Value, name string) error {
	if block.IsZero() {
		return ctx.Raise(object.LocalJumpError, "no block given (%s)", name)
	}
	return nil
}

// toList reads an iterable argument as a list of elements.
func ToList(ctx object.Context, v object.Value) ([]object.Value, error) {
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
	if v.Kind() == object.KindInstance {
		return eachElems(ctx, v)
	}
	return nil, ctx.Raise(object.TypeError, "no implicit conversion of %s into List", KindName(v))
}

// eachElems collects what an object's own each yields, as Ruby's
// Enumerable does for a class that includes it.
func eachElems(ctx object.Context, v object.Value) ([]object.Value, error) {
	var out []object.Value
	collect := ctx.Block(func(args []object.Value) (object.Value, error) {
		out = append(out, yielded(args))
		return object.Nil, nil
	})
	if _, err := ctx.CallWith(v, "each", nil, nil, collect); err != nil {
		return nil, err
	}
	return out, nil
}

// yielded is the one element a yield of args makes: nil for none, the value
// for one, a list of them for several.
func yielded(args []object.Value) object.Value {
	switch len(args) {
	case 0:
		return object.Nil
	case 1:
		return args[0]
	}
	return object.NewList(args...)
}

// StringList returns an Array of the strings.
func StringList(ss []string) object.Value {
	out := make([]object.Value, len(ss))
	for i, s := range ss {
		out[i] = object.String(s)
	}
	return object.NewList(out...)
}
