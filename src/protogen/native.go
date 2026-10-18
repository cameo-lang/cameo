package protogen

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/radiospiel/cameo/src/object"
)

// A native class is one a Go host registered with the message type of a
// generated protobuf message: its descriptor is that message's, not one
// built from its fields, and its instances leave cameo as that Go type.
type native struct {
	md        protoreflect.MessageDescriptor
	prototype proto.Message
}

var (
	nativeMu      sync.RWMutex
	nativeClasses = map[*object.Class]*native{}
	nativeByName  = map[protoreflect.FullName]*object.Class{}
)

// RegisterNative records cls as the class of the message prototype is.
func RegisterNative(cls *object.Class, prototype proto.Message) {
	md := prototype.ProtoReflect().Descriptor()
	nativeMu.Lock()
	defer nativeMu.Unlock()
	nativeClasses[cls] = &native{md: md, prototype: prototype}
	nativeByName[md.FullName()] = cls
}

// NativeClass is the class registered for the message name, nil when there
// is none.
func NativeClass(name protoreflect.FullName) *object.Class {
	nativeMu.RLock()
	defer nativeMu.RUnlock()
	return nativeByName[name]
}

func nativeOf(cls *object.Class) *native {
	nativeMu.RLock()
	defer nativeMu.RUnlock()
	return nativeClasses[cls]
}

// fullName is the protobuf name of the message a class is.
func fullName(cls *object.Class) string {
	if n := nativeOf(cls); n != nil {
		return string(n.md.FullName())
	}
	return Package(cls) + "." + cls.Name
}

// GoMessage is the message an instance is, as Go sees it: a value of the
// registered Go type for a native class, a dynamic message otherwise. It is
// a copy the caller may change.
func (r *Registry) GoMessage(inst *object.Instance) (proto.Message, error) {
	msg, err := r.ToMessage(inst)
	if err != nil {
		return nil, err
	}
	n := nativeOf(inst.Cls)
	if n == nil {
		return proto.Clone(msg), nil
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	out := n.prototype.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Fields are the fields of a native class, one per field of its message:
// the scalar types, repeated fields as lists, maps with a scalar key,
// enums whose values are numbered from 0 without a gap, and messages of
// classes resolve finds, the class itself among them.
func Fields(cls *object.Class, md protoreflect.MessageDescriptor, resolve func(protoreflect.FullName) *object.Class) ([]*object.Field, error) {
	var out []*object.Field
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if o := fd.ContainingOneof(); o != nil && !o.IsSynthetic() {
			return nil, fmt.Errorf("%s: the oneof %s is not supported", fd.FullName(), o.Name())
		}
		var t *object.FieldType
		var err error
		switch {
		case fd.IsMap():
			key := scalarOf(fd.MapKey())
			if key == "" || !object.Scalar(key).MapKey() {
				return nil, fmt.Errorf("%s: a map key must be a string, an integer or a bool", fd.FullName())
			}
			var elem *object.FieldType
			elem, err = elemType(cls, md, fd.MapValue(), resolve)
			t = &object.FieldType{Kind: "map", Key: key, Elem: elem}
		case fd.IsList():
			var elem *object.FieldType
			elem, err = elemType(cls, md, fd, resolve)
			t = &object.FieldType{Kind: "list", Elem: elem}
		default:
			t, err = elemType(cls, md, fd, resolve)
		}
		if err != nil {
			return nil, err
		}
		f := &object.Field{Name: string(fd.Name()), Number: int64(fd.Number()), Type: t, Owner: cls}
		if t.Scalar != nil && !fd.HasPresence() {
			// A field without presence that is not set is its zero value,
			// so a new instance starts there, as the Go message does.
			v, err := scalarFromProto(t.Scalar, fd.Default())
			if err != nil {
				return nil, fmt.Errorf("%s: %w", fd.FullName(), err)
			}
			f.Default = v
		}
		out = append(out, f)
	}
	return out, nil
}

func elemType(cls *object.Class, md protoreflect.MessageDescriptor, fd protoreflect.FieldDescriptor, resolve func(protoreflect.FullName) *object.Class) (*object.FieldType, error) {
	if name := scalarOf(fd); name != "" {
		return object.ScalarField(name), nil
	}
	switch fd.Kind() {
	case protoreflect.EnumKind:
		return enumType(fd.Enum())
	case protoreflect.MessageKind:
		name := fd.Message().FullName()
		if name == md.FullName() {
			return &object.FieldType{Kind: "ref", Ref: cls}, nil
		}
		ref := resolve(name)
		if ref == nil {
			return nil, fmt.Errorf("%s: register the class of %s first", fd.FullName(), name)
		}
		return &object.FieldType{Kind: "ref", Ref: ref}, nil
	}
	return nil, fmt.Errorf("%s: a %s field is not supported", fd.FullName(), fd.Kind())
}

// enumType is an enum field's type: the symbols of the values 1 to n, in
// lower case and without the prefix all value names share; 0 is nil, as
// in a class's own enum field.
func enumType(ed protoreflect.EnumDescriptor) (*object.FieldType, error) {
	values := ed.Values()
	names := make([]string, values.Len())
	for i := range names {
		v := values.Get(i)
		if int(v.Number()) < 0 || int(v.Number()) >= len(names) || names[v.Number()] != "" {
			return nil, fmt.Errorf("%s: an enum's values must be numbered 0 to %d", ed.FullName(), len(names)-1)
		}
		names[v.Number()] = string(v.Name())
	}
	prefix := commonPrefix(names)
	syms := make([]string, len(names)-1)
	for i, n := range names[1:] {
		syms[i] = strings.ToLower(strings.TrimPrefix(n, prefix))
	}
	if len(syms) == 0 {
		return nil, fmt.Errorf("%s: an enum needs a value besides 0", ed.FullName())
	}
	return &object.FieldType{Kind: "enum", Enum: syms}, nil
}

// commonPrefix is the longest prefix ending in _ that every name has:
// COLOR_ of COLOR_UNSPECIFIED and COLOR_RED.
func commonPrefix(names []string) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	first, last := sorted[0], sorted[len(sorted)-1]
	n := 0
	for n < len(first) && n < len(last) && first[n] == last[n] {
		n++
	}
	if i := strings.LastIndexByte(first[:n], '_'); i >= 0 {
		return first[:i+1]
	}
	return ""
}

// scalarOf is the cameo scalar type of a protobuf scalar field, "" for a
// field of another kind.
func scalarOf(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "bytes"
	case protoreflect.DoubleKind:
		return "double"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return fd.Kind().String()
	}
	return ""
}
