package eval

import (
	"strconv"
	"strings"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/stdlib"
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
		r, err := in.invoke(m, v, nil, nil, object.Value{}, 0)
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
		r, err := in.invoke(m, v, nil, nil, object.Value{}, 0)
		if err != nil {
			return "", err
		}
		if s, ok := r.AsString(); ok {
			return string(s), nil
		}
		return in.inspectBuiltin(r)
	}
	switch v.Kind() {
	case object.KindString:
		v := v.Str()
		return string(v), nil
	case object.KindSymbol:
		v := v.Str()
		return string(v), nil
	case object.KindException:
		v := v.Exception()
		return v.Message, nil
	case object.KindTime:
		return stdlib.TimeToS(v.Time()), nil
	case object.KindInstance:
		switch v.Instance().Cls {
		case in.mainClass:
			return "main", nil
		case object.ObjectClass:
			return "#<Object>", nil
		}
	}
	if v.IsNil() {
		return "", nil
	}
	return in.inspectBuiltin(v)
}

func (in *Interp) inspectBuiltin(v object.Value) (string, error) {
	switch v.Kind() {
	case object.KindBool:
		v := v.Bool()
		if v {
			return "true", nil
		}
		return "false", nil
	case object.KindInt:
		v := v.Int()
		return itoa(int64(v)), nil
	case object.KindFloat:
		v := v.Float()
		return object.FormatFloat(float64(v)), nil
	case object.KindString:
		v := v.Str()
		return object.QuoteString(string(v)), nil
	case object.KindSymbol:
		v := v.Str()
		return object.SymbolInspect(string(v)), nil
	case object.KindBytes:
		v := v.Bytes()
		return object.QuoteBytes(v) + ".b", nil
	case object.KindTime:
		return stdlib.TimeInspect(v.Time()), nil
	case object.KindList:
		v := v.List()
		parts := make([]string, len(v.Elems))
		for i, e := range v.Elems {
			s, err := in.Inspect(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case object.KindMap:
		v := v.Map()
		if v.Len() == 0 {
			return "{}", nil
		}
		parts := make([]string, v.Len())
		for i, k := range v.Keys() {
			val, err := in.Inspect(v.Values()[i])
			if err != nil {
				return "", err
			}
			if s, ok := k.AsSymbol(); ok && object.SymbolInspect(string(s)) == ":"+string(s) {
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
	case object.KindSet:
		v := v.Set()
		parts := make([]string, v.Len())
		for i, e := range v.Elems() {
			s, err := in.Inspect(e)
			if err != nil {
				return "", err
			}
			parts[i] = s
		}
		return "#<Set: {" + strings.Join(parts, ", ") + "}>", nil
	case object.KindRange:
		v := v.Range()
		lo, hi := "", ""
		var err error
		if !v.Lo.IsZero() {
			if lo, err = in.Inspect(v.Lo); err != nil {
				return "", err
			}
		}
		if !v.Hi.IsZero() {
			if hi, err = in.Inspect(v.Hi); err != nil {
				return "", err
			}
		}
		if v.Exclusive {
			return lo + "..." + hi, nil
		}
		return lo + ".." + hi, nil
	case object.KindClass:
		v := v.Cls()
		return v.QualifiedName(), nil
	case object.KindException:
		v := v.Exception()
		return "#<" + v.Cls.QualifiedName() + ": " + v.Message + ">", nil
	case object.KindInstance:
		v := v.Instance()
		switch v.Cls {
		case in.mainClass:
			return "main", nil
		case object.ObjectClass:
			// A bare Object has no fields to show, and amber no address.
			return "#<Object>", nil
		}
		parts := make([]string, len(v.Cls.Fields))
		for i, f := range v.Cls.Fields {
			s, err := in.Inspect(object.OrNil(v.Fields[i]))
			if err != nil {
				return "", err
			}
			parts[i] = f.Name + ": " + s
		}
		return v.Cls.QualifiedName() + "(" + strings.Join(parts, ", ") + ")", nil
	case object.KindProc:
		v := proc(v)
		if v.Strict {
			return "#<Proc (lambda)>", nil
		}
		return "#<Proc>", nil
	}
	if v.IsNil() {
		return "nil", nil
	}
	return "#<" + v.Class().QualifiedName() + ">", nil
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// Equal implements object.Context: structural equality, through a
// user-defined == where a class has one.
func (in *Interp) Equal(a, b object.Value) (bool, error) {
	if m := userMethod(a, "=="); m != nil {
		r, err := in.invoke(m, a, []object.Value{b}, nil, object.Value{}, 0)
		if err != nil {
			return false, err
		}
		return r.Truthy(), nil
	}
	switch a.Kind() {
	case object.KindList:
		a := a.List()
		bl, ok := b.AsList()
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
	case object.KindMap:
		a := a.Map()
		bm, ok := b.AsMap()
		if !ok || a.Len() != bm.Len() {
			return false, nil
		}
		for i, k := range a.Keys() {
			bv, found := bm.Get(k)
			if !found {
				return false, nil
			}
			eq, err := in.Equal(a.Values()[i], bv)
			if err != nil || !eq {
				return false, err
			}
		}
		return true, nil
	case object.KindInstance:
		a := a.Instance()
		bi, ok := b.AsInstance()
		if !ok || a.Cls != bi.Cls {
			return false, nil
		}
		for i := range a.Fields {
			eq, err := in.Equal(object.OrNil(a.Fields[i]), object.OrNil(bi.Fields[i]))
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
		r, err := in.invoke(m, a, []object.Value{b}, nil, object.Value{}, 0)
		if err != nil {
			return 0, err
		}
		if i, ok := r.AsInt(); ok {
			return int(i), nil
		}
		return 0, in.incomparable(a, b)
	}
	switch a.Kind() {
	case object.KindInt:
		switch b.Kind() {
		case object.KindInt:
			return cmp(a.Int(), b.Int()), nil
		case object.KindFloat:
			return cmpFloat(float64(a.Int()), b.Float()), nil
		}
	case object.KindFloat:
		if y, ok := b.AsNumber(); ok {
			return cmpFloat(a.Float(), y), nil
		}
	case object.KindString:
		a := a.Str()
		if b, ok := b.AsString(); ok {
			return strings.Compare(string(a), string(b)), nil
		}
	case object.KindSymbol:
		a := a.Str()
		if b, ok := b.AsSymbol(); ok {
			return strings.Compare(string(a), string(b)), nil
		}
	case object.KindTime:
		if b, ok := b.AsTime(); ok {
			return a.Time().Compare(b), nil
		}
	case object.KindList:
		a := a.List()
		if bl, ok := b.AsList(); ok {
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
	if v.IsNil() {
		return "nil"
	}
	if b, ok := v.AsBool(); ok {
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
	if !r.Lo.IsZero() {
		c, err := in.Compare(r.Lo, v)
		if err != nil {
			return false, nil
		}
		if c > 0 {
			return false, nil
		}
	}
	if !r.Hi.IsZero() {
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

// rangeToList expands a range of integers, strings or symbols.
func (in *Interp) rangeToList(r *object.Range, line int) (*object.List, error) {
	elems, err := stdlib.RangeElems(in, r)
	if err != nil {
		return nil, withLine(err, line)
	}
	return object.NewList(elems...).List(), nil
}
