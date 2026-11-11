package eval

import (
	"github.com/cameo-lang/cameo/src/object"
)

// deconstruct returns a value's elements for a list pattern: a list's
// elements, an instance's fields.
func (in *Interp) deconstruct(v object.Value) ([]object.Value, bool) {
	switch v.Kind() {
	case object.KindList:
		v := v.List()
		return v.Elems, true
	case object.KindInstance:
		v := v.Instance()
		out := make([]object.Value, len(v.Fields))
		for i, f := range v.Fields {
			out[i] = object.OrNil(f)
		}
		return out, true
	}
	return nil, false
}

// deconstructKeys returns a value as a map for a map pattern: a map itself,
// an instance's fields by name, a Time's parts.
func (in *Interp) deconstructKeys(v object.Value) (*object.Map, bool) {
	switch v.Kind() {
	case object.KindMap:
		v := v.Map()
		return v, true
	case object.KindInstance:
		v := v.Instance()
		b := object.NewMapBuilder()
		for i, f := range v.Cls.Fields {
			b.Set(object.Symbol(f.Name), object.OrNil(v.Fields[i]))
		}
		return b.Map(), true
	case object.KindTime:
		// Time#deconstruct_keys(nil) is native and cannot fail.
		r, err := object.TimeClass.Methods["deconstruct_keys"].Native(in, v, []object.Value{object.Nil}, nil, object.Value{})
		if err != nil {
			return nil, false
		}
		return r.Map(), true
	}
	return nil, false
}
