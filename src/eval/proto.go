package eval

import (
	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/protogen"
)

// Proto is the interpreter's protobuf registry: the descriptors of its
// classes, and encoders over them.
func (in *Interp) Proto() *protogen.Registry { return in.proto }

// Encoder serialises values into the envelope.
func (in *Interp) Encoder() *protogen.Encoder { return protogen.NewEncoder(in.proto) }

// Decoder reads values from the envelope, finding classes by their qualified
// name and digest among the interpreter's constants.
func (in *Interp) Decoder() *protogen.Decoder {
	return protogen.NewDecoder(in.proto, in.FindClass)
}

// FindClass finds a loaded class by qualified name (Geo::Point) and digest;
// nil when none matches.
func (in *Interp) FindClass(name, digest string) *object.Class {
	var walk func(consts map[string]object.Value, prefix string) *object.Class
	walk = func(consts map[string]object.Value, prefix string) *object.Class {
		for n, v := range consts {
			cls, ok := v.(*object.Class)
			if !ok {
				continue
			}
			if prefix+n == name && cls.Digest == digest {
				return cls
			}
			if found := walk(cls.Consts, prefix+n+"::"); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(in.consts, "")
}

// Classes lists every user class the script defined, in definition order of
// their names.
func (in *Interp) Classes() []*object.Class {
	var out []*object.Class
	var walk func(consts map[string]object.Value)
	walk = func(consts map[string]object.Value) {
		for _, name := range sortedKeys(consts) {
			cls, ok := consts[name].(*object.Class)
			if !ok || cls.Digest == "" {
				continue
			}
			if !cls.IsModule {
				out = append(out, cls)
			}
			walk(cls.Consts)
		}
	}
	walk(in.consts)
	return out
}

func sortedKeys(m map[string]object.Value) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func init() {
	cls := object.ClassClass
	cls.Define("proto", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c := self.(*object.Class)
		if c.Digest == "" || c.IsModule {
			return nil, ctx.Raise(object.TypeError, "%s is not a class with fields", c.QualifiedName())
		}
		if _, err := ctx.(*Interp).proto.Message(c); err != nil {
			return nil, ctx.Raise(object.AmberError, "%s", err.Error())
		}
		return object.String(protogen.Text(c)), nil
	})
	cls.Define("from_proto", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c := self.(*object.Class)
		var data []byte
		switch v := args[0].(type) {
		case object.Bytes:
			data = v
		case object.String:
			data = []byte(v)
		default:
			return nil, ctx.Raise(object.TypeError, "from_proto takes bytes, not %s", typeName(args[0]))
		}
		inst, err := ctx.(*Interp).proto.Unmarshal(c, data)
		if err != nil {
			return nil, ctx.Raise(object.ArgumentError, "%s", err.Error())
		}
		return inst, nil
	})
	object.ObjectClass.Define("to_proto", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst, ok := self.(*object.Instance)
		if !ok || inst.Cls.Digest == "" || inst.Cls == ctx.(*Interp).mainClass {
			return nil, ctx.Raise(object.TypeError, "%s is not an instance of a class with fields", typeName(self))
		}
		data, err := ctx.(*Interp).proto.Marshal(inst)
		if err != nil {
			return nil, ctx.Raise(object.AmberError, "%s", err.Error())
		}
		return object.Bytes(data), nil
	})
	object.ObjectClass.Define("to_value_proto", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		data, err := ctx.(*Interp).Encoder().Encode(self)
		if err != nil {
			return nil, ctx.Raise(object.TypeError, "%s", err.Error())
		}
		return object.Bytes(data), nil
	})
	object.ObjectClass.Define("from_value_proto", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		data, ok := args[0].(object.Bytes)
		if !ok {
			return nil, ctx.Raise(object.TypeError, "from_value_proto takes bytes, not %s", typeName(args[0]))
		}
		v, err := ctx.(*Interp).Decoder().Decode(data)
		if err != nil {
			return nil, ctx.Raise(object.ArgumentError, "%s", err.Error())
		}
		return v, nil
	})
}
