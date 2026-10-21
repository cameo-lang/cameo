package eval

import (
	"github.com/cameo-lang/cameo/src/object"
)

// Exceptions: Exception.new, which runs a class's initialize on the
// exception it makes. The design is in doc/design/exceptions.md.

// newException is Exception.new: an exception with the class's name for
// its message and its fields at their defaults, which initialize sets, the
// class's own or Exception's. A class without an initialize of its own
// takes its fields as keywords, and the message and the rest of what
// Ruby's exceptions take as Exception's initialize does.
func (in *Interp) newException(cls *object.Class, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	exc := &object.Exception{Cls: cls, Message: cls.QualifiedName()}
	if len(cls.Fields) > 0 {
		exc.Fields = make([]object.Value, len(cls.Fields))
		for i, f := range cls.Fields {
			exc.Fields[i] = f.Default
			if f.Default.IsZero() {
				exc.Fields[i] = emptyFor(f.Type)
			}
		}
	}
	m := cls.Lookup("initialize")
	self := object.ExceptionValue(exc)
	exc.Building = true
	defer func() { exc.Building = false }()
	if m.Body != nil {
		if _, err := in.invoke(m, self, args, kw, block, 0); err != nil {
			return object.Value{}, err
		}
		return self, nil
	}
	if len(cls.Fields) > 0 {
		// One positional map stands for the keywords: what a Go host passes.
		if len(args) == 1 && kw == nil {
			if m, ok := args[0].AsMap(); ok {
				kw, args = m, nil
			}
		}
		if kw != nil {
			rest := object.NewMapBuilder()
			others := false
			for i, k := range kw.Keys() {
				name, _ := k.AsSymbol()
				if s, ok := k.AsString(); ok {
					name = s
				}
				idx, ok := cls.FieldIndex[name]
				if !ok {
					rest.Set(k, kw.Values()[i])
					others = true
					continue
				}
				v, err := in.checkField(cls.Fields[idx], kw.Values()[i], 0)
				if err != nil {
					return object.Value{}, err
				}
				exc.Fields[idx] = v
			}
			kw = nil
			if others {
				kw = rest.Map()
			}
		}
	}
	if _, err := m.Native(in, self, args, kw, block); err != nil {
		return object.Value{}, err
	}
	return self, nil
}
