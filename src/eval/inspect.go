package eval

import (
	"strconv"
	"strings"

	"github.com/radiospiel/amber/src/object"
)

// userMethod finds a user-defined (not native) method name on v's class.
func userMethod(v object.Value, name string) *object.Method {
	m := v.Class().Lookup(name)
	if m != nil && m.Body != nil {
		return m
	}
	return nil
}

// Inspect implements object.Context: a value's inspect, as p prints it.
func (in *Interp) Inspect(v object.Value) (string, error) {
	if m := userMethod(v, "inspect"); m != nil {
		r, err := in.invoke(m, v, nil, nil, nil, 0)
		if err != nil {
			return "", err
		}
		return in.ToS(r)
	}
	return in.inspectBuiltin(v)
}

// ToS implements object.Context: a value's to_s, as puts prints it.
func (in *Interp) ToS(v object.Value) (string, error) {
	if m := userMethod(v, "to_s"); m != nil {
		r, err := in.invoke(m, v, nil, nil, nil, 0)
		if err != nil {
			return "", err
		}
		if s, ok := r.(object.String); ok {
			return string(s), nil
		}
		return in.inspectBuiltin(r)
	}
	switch v := v.(type) {
	case object.String:
		return string(v), nil
	case object.Symbol:
		return string(v), nil
	case *object.Exception:
		return v.Message, nil
	case *object.Instance:
		if v.Cls == object.ObjectClass {
			return "main", nil
		}
	}
	if v == object.Nil {
		return "", nil
	}
	return in.inspectBuiltin(v)
}

func (in *Interp) inspectBuiltin(v object.Value) (string, error) {
	switch v := v.(type) {
	case object.Bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case object.Int:
		return itoa(int64(v)), nil
	case object.Float:
		return object.FormatFloat(float64(v)), nil
	case object.String:
		return object.QuoteString(string(v)), nil
	case object.Symbol:
		return object.SymbolInspect(string(v)), nil
	case object.Bytes:
		return object.QuoteString(string(v)) + ".b", nil
	case *object.List:
		parts := make([]string, len(v.Elems))
		for i, e := range v.Elems {
			s, err := in.Inspect(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case *object.Map:
		if v.Len() == 0 {
			return "{}", nil
		}
		parts := make([]string, v.Len())
		for i, k := range v.Keys() {
			val, err := in.Inspect(v.Values()[i])
			if err != nil {
				return "", err
			}
			if s, ok := k.(object.Symbol); ok && object.SymbolInspect(string(s)) == ":"+string(s) {
				parts[i] = string(s) + ": " + val
				continue
			}
			ks, err := in.Inspect(k)
			if err != nil {
				return "", err
			}
			parts[i] = ks + " => " + val
		}
		return "{" + strings.Join(parts, ", ") + "}", nil
	case *object.Set:
		parts := make([]string, v.Len())
		for i, e := range v.Elems() {
			s, err := in.Inspect(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return "#<Set: {" + strings.Join(parts, ", ") + "}>", nil
	case *object.Range:
		lo, hi := "", ""
		var err error
		if v.Lo != nil {
			if lo, err = in.Inspect(v.Lo); err != nil {
				return "", err
			}
		}
		if v.Hi != nil {
			if hi, err = in.Inspect(v.Hi); err != nil {
				return "", err
			}
		}
		if v.Exclusive {
			return lo + "..." + hi, nil
		}
		return lo + ".." + hi, nil
	case *object.Class:
		return v.QualifiedName(), nil
	case *object.Exception:
		return "#<" + v.Cls.QualifiedName() + ": " + v.Message + ">", nil
	case *object.Instance:
		if v.Cls == object.ObjectClass {
			return "main", nil
		}
		parts := make([]string, len(v.Cls.Fields))
		for i, f := range v.Cls.Fields {
			s, err := in.Inspect(orNil(v.Fields[i]))
			if err != nil {
				return "", err
			}
			parts[i] = f.Name + ": " + s
		}
		return v.Cls.QualifiedName() + "(" + strings.Join(parts, ", ") + ")", nil
	case *Proc:
		if v.Strict {
			return "#<Proc (lambda)>", nil
		}
		return "#<Proc>", nil
	}
	if v == object.Nil {
		return "nil", nil
	}
	return "#<" + v.Class().QualifiedName() + ">", nil
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// Equal implements object.Context: structural equality, through a
// user-defined == where a class has one.
func (in *Interp) Equal(a, b object.Value) (bool, error) {
	if m := userMethod(a, "=="); m != nil {
		r, err := in.invoke(m, a, []object.Value{b}, nil, nil, 0)
		if err != nil {
			return false, err
		}
		return object.Truthy(r), nil
	}
	switch a := a.(type) {
	case *object.List:
		bl, ok := b.(*object.List)
		if !ok || len(a.Elems) != len(bl.Elems) {
			return false, nil
		}
		for i := range a.Elems {
			eq, err := in.Equal(a.Elems[i], bl.Elems[i])
			if err != nil || !eq {
				return false, err
			}
		}
		return true, nil
	case *object.Instance:
		bi, ok := b.(*object.Instance)
		if !ok || a.Cls != bi.Cls {
			return false, nil
		}
		for i := range a.Fields {
			eq, err := in.Equal(orNil(a.Fields[i]), orNil(bi.Fields[i]))
			if err != nil || !eq {
				return false, err
			}
		}
		return true, nil
	}
	return object.Equal(a, b), nil
}

// Compare implements object.Context: <=>, through a user-defined one where a
// class has it. It fails with an ArgumentError when the values do not
// compare.
func (in *Interp) Compare(a, b object.Value) (int, error) {
	if m := userMethod(a, "<=>"); m != nil {
		r, err := in.invoke(m, a, []object.Value{b}, nil, nil, 0)
		if err != nil {
			return 0, err
		}
		if i, ok := r.(object.Int); ok {
			return int(i), nil
		}
		return 0, in.incomparable(a, b)
	}
	switch a := a.(type) {
	case object.Int:
		switch b := b.(type) {
		case object.Int:
			return cmp(int64(a), int64(b)), nil
		case object.Float:
			return cmpFloat(float64(a), float64(b)), nil
		}
	case object.Float:
		switch b := b.(type) {
		case object.Int:
			return cmpFloat(float64(a), float64(b)), nil
		case object.Float:
			return cmpFloat(float64(a), float64(b)), nil
		}
	case object.String:
		if b, ok := b.(object.String); ok {
			return strings.Compare(string(a), string(b)), nil
		}
	case object.Symbol:
		if b, ok := b.(object.Symbol); ok {
			return strings.Compare(string(a), string(b)), nil
		}
	case *object.List:
		if bl, ok := b.(*object.List); ok {
			for i := 0; i < len(a.Elems) && i < len(bl.Elems); i++ {
				c, err := in.Compare(a.Elems[i], bl.Elems[i])
				if err != nil || c != 0 {
					return c, err
				}
			}
			return cmp(int64(len(a.Elems)), int64(len(bl.Elems))), nil
		}
	}
	return 0, in.incomparable(a, b)
}

func (in *Interp) incomparable(a, b object.Value) error {
	return in.raise(object.ArgumentError, 0, "comparison of %s with %s failed", kindName(a), kindName(b))
}

// kindName names a value's class the way Ruby's comparison errors do.
func kindName(v object.Value) string {
	if v == object.Nil {
		return "nil"
	}
	if b, ok := v.(object.Bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return v.Class().QualifiedName()
}

func cmp(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// rangeIncludes is Range#include? and `range === v`.
func (in *Interp) rangeIncludes(r *object.Range, v object.Value) (bool, error) {
	if r.Lo != nil {
		c, err := in.Compare(r.Lo, v)
		if err != nil {
			return false, nil
		}
		if c > 0 {
			return false, nil
		}
	}
	if r.Hi != nil {
		c, err := in.Compare(v, r.Hi)
		if err != nil {
			return false, nil
		}
		if c > 0 || (c == 0 && r.Exclusive) {
			return false, nil
		}
	}
	return true, nil
}

// rangeToList expands an integer range.
func (in *Interp) rangeToList(r *object.Range, line int) (*object.List, error) {
	lo, ok1 := r.Lo.(object.Int)
	hi, ok2 := r.Hi.(object.Int)
	if !ok1 || !ok2 {
		if r.Hi == nil {
			return nil, in.raise(object.RangeError, line, "cannot convert endless range to a list")
		}
		return nil, in.raise(object.TypeError, line, "can't iterate from %s", typeName(orNil(r.Lo)))
	}
	if r.Exclusive {
		hi--
	}
	elems := []object.Value{}
	for i := lo; i <= hi; i++ {
		elems = append(elems, i)
	}
	return object.NewList(elems...), nil
}
