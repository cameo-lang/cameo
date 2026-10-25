package eval

import (
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
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

// FindClass finds a loaded or registered class by qualified name
// (Geo::Point) and digest; nil when none matches. It follows the name's
// path rather than walking every constant, which may refer back to a class
// that holds it.
func (in *Interp) FindClass(name, digest string) *object.Class {
	find := func(consts map[string]object.Value) *object.Class {
		parts := strings.Split(name, "::")
		for i, p := range parts {
			cls, ok := consts[p].AsClass()
			if !ok {
				return nil
			}
			if i == len(parts)-1 {
				if cls.Digest == digest {
					return cls
				}
				return nil
			}
			consts = cls.Consts
		}
		return nil
	}
	if cls := find(in.consts); cls != nil {
		return cls
	}
	// A class a Go host registered is a constant beside the builtin ones.
	return find(object.ObjectClass.Consts)
}

// Classes lists every user class the script defined, in definition order of
// their names.
func (in *Interp) Classes() []*object.Class {
	var out []*object.Class
	var walk func(consts map[string]object.Value)
	walk = func(consts map[string]object.Value) {
		for _, name := range sortedKeys(consts) {
			cls, ok := consts[name].AsClass()
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

// HostExceptions lists the exception classes the host defined in its
// modules (Module.Exception). They have no digest, so Classes leaves
// them out.
func (in *Interp) HostExceptions() []*object.Class {
	var out []*object.Class
	for _, name := range sortedKeys(in.consts) {
		if !hostDefined(in.consts[name]) {
			continue
		}
		mod, _ := in.consts[name].AsClass()
		for _, inner := range sortedKeys(mod.Consts) {
			if cls, ok := mod.Consts[inner].AsClass(); ok && !cls.IsModule && cls.IsA(object.ExceptionClass) {
				out = append(out, cls)
			}
		}
	}
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
		c := self.Cls()
		if c.Digest == "" && !c.IsA(object.ExceptionClass) || c.IsModule {
			return object.Value{}, ctx.Raise(object.TypeError, "%s is not a class with fields", c.QualifiedName())
		}
		if _, err := ctx.(*Interp).proto.Message(c); err != nil {
			return object.Value{}, ctx.Raise(object.CameoError, "%s", err.Error())
		}
		return object.String(protogen.Text(c)), nil
	})
	cls.Define("from_proto", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		c := self.Cls()
		var data []byte
		switch args[0].Kind() {
		case object.KindBytes:
			v := args[0].Bytes()
			data = v
		case object.KindString, object.KindASCII:
			v := args[0].Str()
			data = []byte(v)
		default:
			return object.Value{}, ctx.Raise(object.TypeError, "from_proto takes bytes, not %s", typeName(args[0]))
		}
		if c.IsA(object.ExceptionClass) {
			exc, err := ctx.(*Interp).Decoder().UnmarshalException(c, data)
			if err != nil {
				return object.Value{}, ctx.Raise(object.ArgumentError, "%s", err.Error())
			}
			return object.ExceptionValue(exc), nil
		}
		inst, err := ctx.(*Interp).proto.Unmarshal(c, data)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "%s", err.Error())
		}
		return object.InstanceValue(inst), nil
	})
	object.ObjectClass.Define("to_proto", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if exc, ok := self.AsException(); ok {
			data, err := ctx.(*Interp).proto.MarshalException(exc)
			if err != nil {
				return object.Value{}, ctx.Raise(object.CameoError, "%s", err.Error())
			}
			return object.Bytes(data), nil
		}
		inst, ok := self.AsInstance()
		if !ok || inst.Cls.Digest == "" || inst.Cls == ctx.(*Interp).mainClass {
			return object.Value{}, ctx.Raise(object.TypeError, "%s is not an instance of a class with fields", typeName(self))
		}
		data, err := ctx.(*Interp).proto.Marshal(inst)
		if err != nil {
			return object.Value{}, ctx.Raise(object.CameoError, "%s", err.Error())
		}
		return object.Bytes(data), nil
	})
	object.ObjectClass.Define("to_value_proto", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		data, err := ctx.(*Interp).Encoder().Encode(self)
		if err != nil {
			return object.Value{}, ctx.Raise(object.TypeError, "%s", err.Error())
		}
		return object.Bytes(data), nil
	})
	object.ObjectClass.Define("from_value_proto", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		data, ok := args[0].AsBytes()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "from_value_proto takes bytes, not %s", typeName(args[0]))
		}
		v, err := ctx.(*Interp).Decoder().Decode(data)
		if err != nil {
			return object.Value{}, ctx.Raise(object.ArgumentError, "%s", err.Error())
		}
		return v, nil
	})
}
