package protogen

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/radiospiel/amber/src/object"
)

// The envelope: amber.Value carries any serialisable value. It is what async
// serialises and what the host API takes and returns.
//
//	message Value {
//	  oneof kind {
//	    bool bool_value = 1; int64 int_value = 2; double float_value = 3;
//	    string string_value = 4; bytes bytes_value = 5;
//	    List list = 6; Map map = 7; Set set = 8; Range range = 9; Object object = 10;
//	  }
//	}
//	message List { repeated Value items = 1; }
//	message Set { repeated Value items = 1; }
//	message Map { repeated Entry entries = 1; message Entry { Value key = 1; Value value = 2; } }
//	message Range { Value lo = 1; Value hi = 2; bool exclusive = 3; }
//	message Object { string class = 1; string digest = 2; bytes message = 3; }
//
// A Value with no kind set is nil. A symbol travels as a string.
var (
	envelopeOnce sync.Once
	envelopeFile protoreflect.FileDescriptor
	envelopeErr  error
)

// EnvelopeText is the .proto text of the envelope.
const EnvelopeText = `message Value {
  oneof kind {
    bool bool_value = 1;
    int64 int_value = 2;
    double float_value = 3;
    string string_value = 4;
    bytes bytes_value = 5;
    List list = 6;
    Map map = 7;
    Set set = 8;
    Range range = 9;
    Object object = 10;
  }
}

message List {
  repeated Value items = 1;
}

message Set {
  repeated Value items = 1;
}

message Map {
  message Entry {
    Value key = 1;
    Value value = 2;
  }
  repeated Entry entries = 1;
}

message Range {
  Value lo = 1;
  Value hi = 2;
  bool exclusive = 3;
}

message Object {
  string class = 1;
  string digest = 2;
  bytes message = 3;
}
`

func envelope() (protoreflect.FileDescriptor, error) {
	envelopeOnce.Do(func() {
		opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
		rep := descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		field := func(name string, num int32, typ descriptorpb.FieldDescriptorProto_Type, typeName string, label *descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
			f := &descriptorpb.FieldDescriptorProto{Name: strPtr(name), JsonName: strPtr(jsonName(name)), Number: int32Ptr(num), Type: typ.Enum(), Label: label}
			if typeName != "" {
				f.TypeName = strPtr(".amber." + typeName)
			}
			return f
		}
		oneof := func(f *descriptorpb.FieldDescriptorProto) *descriptorpb.FieldDescriptorProto {
			f.OneofIndex = int32Ptr(0)
			return f
		}
		value := &descriptorpb.DescriptorProto{
			Name:      strPtr("Value"),
			OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: strPtr("kind")}},
			Field: []*descriptorpb.FieldDescriptorProto{
				oneof(field("bool_value", 1, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", opt)),
				oneof(field("int_value", 2, descriptorpb.FieldDescriptorProto_TYPE_INT64, "", opt)),
				oneof(field("float_value", 3, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "", opt)),
				oneof(field("string_value", 4, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", opt)),
				oneof(field("bytes_value", 5, descriptorpb.FieldDescriptorProto_TYPE_BYTES, "", opt)),
				oneof(field("list", 6, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "List", opt)),
				oneof(field("map", 7, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Map", opt)),
				oneof(field("set", 8, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Set", opt)),
				oneof(field("range", 9, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Range", opt)),
				oneof(field("object", 10, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Object", opt)),
			},
		}
		items := func(name string) *descriptorpb.DescriptorProto {
			return &descriptorpb.DescriptorProto{Name: strPtr(name), Field: []*descriptorpb.FieldDescriptorProto{field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Value", rep)}}
		}
		entry := &descriptorpb.DescriptorProto{Name: strPtr("Entry"), Field: []*descriptorpb.FieldDescriptorProto{
			field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Value", opt),
			field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Value", opt),
		}}
		mapMsg := &descriptorpb.DescriptorProto{Name: strPtr("Map"), NestedType: []*descriptorpb.DescriptorProto{entry},
			Field: []*descriptorpb.FieldDescriptorProto{field("entries", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Map.Entry", rep)}}
		rangeMsg := &descriptorpb.DescriptorProto{Name: strPtr("Range"), Field: []*descriptorpb.FieldDescriptorProto{
			field("lo", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Value", opt),
			field("hi", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "Value", opt),
			field("exclusive", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", opt),
		}}
		objectMsg := &descriptorpb.DescriptorProto{Name: strPtr("Object"), Field: []*descriptorpb.FieldDescriptorProto{
			field("class", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", opt),
			field("digest", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", opt),
			field("message", 3, descriptorpb.FieldDescriptorProto_TYPE_BYTES, "", opt),
		}}
		fdp := &descriptorpb.FileDescriptorProto{
			Name:        strPtr("amber/value.proto"),
			Package:     strPtr("amber"),
			Syntax:      strPtr("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{value, items("List"), items("Set"), mapMsg, rangeMsg, objectMsg},
		}
		envelopeFile, envelopeErr = protodesc.NewFile(fdp, nil)
	})
	return envelopeFile, envelopeErr
}

// Encoder serialises values into the envelope.
type Encoder struct{ reg *Registry }

// NewEncoder makes an encoder over the registry's classes.
func NewEncoder(reg *Registry) *Encoder { return &Encoder{reg: reg} }

// Encode is the wire form of a value in the envelope. A Proc, a class or a
// process cannot be encoded.
func (e *Encoder) Encode(v object.Value) ([]byte, error) {
	msg, err := e.Message(v)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Message is a value as an amber.Value message.
func (e *Encoder) Message(v object.Value) (*dynamicpb.Message, error) {
	fd, err := envelope()
	if err != nil {
		return nil, err
	}
	return e.message(fd, v)
}

func (e *Encoder) message(fd protoreflect.FileDescriptor, v object.Value) (*dynamicpb.Message, error) {
	md := fd.Messages().ByName("Value")
	msg := dynamicpb.NewMessage(md)
	set := func(name string, pv protoreflect.Value) {
		msg.Set(md.Fields().ByName(protoreflect.Name(name)), pv)
	}
	switch v := v.(type) {
	case object.Bool:
		set("bool_value", protoreflect.ValueOfBool(bool(v)))
	case object.Int:
		set("int_value", protoreflect.ValueOfInt64(int64(v)))
	case object.Float:
		set("float_value", protoreflect.ValueOfFloat64(float64(v)))
	case object.String:
		set("string_value", protoreflect.ValueOfString(string(v)))
	case object.Symbol:
		set("string_value", protoreflect.ValueOfString(string(v)))
	case object.Bytes:
		set("bytes_value", protoreflect.ValueOfBytes([]byte(v)))
	case *object.List:
		items, err := e.items(fd, "List", v.Elems)
		if err != nil {
			return nil, err
		}
		set("list", protoreflect.ValueOfMessage(items))
	case *object.Set:
		items, err := e.items(fd, "Set", v.Elems())
		if err != nil {
			return nil, err
		}
		set("set", protoreflect.ValueOfMessage(items))
	case *object.Map:
		mapMd := fd.Messages().ByName("Map")
		entryMd := mapMd.Messages().ByName("Entry")
		m := dynamicpb.NewMessage(mapMd)
		entries := m.Mutable(mapMd.Fields().ByName("entries")).List()
		for i, k := range v.Keys() {
			entry := dynamicpb.NewMessage(entryMd)
			km, err := e.message(fd, k)
			if err != nil {
				return nil, err
			}
			vm, err := e.message(fd, v.Values()[i])
			if err != nil {
				return nil, err
			}
			entry.Set(entryMd.Fields().ByName("key"), protoreflect.ValueOfMessage(km))
			entry.Set(entryMd.Fields().ByName("value"), protoreflect.ValueOfMessage(vm))
			entries.Append(protoreflect.ValueOfMessage(entry))
		}
		set("map", protoreflect.ValueOfMessage(m))
	case *object.Range:
		rangeMd := fd.Messages().ByName("Range")
		r := dynamicpb.NewMessage(rangeMd)
		if v.Lo != nil {
			lo, err := e.message(fd, v.Lo)
			if err != nil {
				return nil, err
			}
			r.Set(rangeMd.Fields().ByName("lo"), protoreflect.ValueOfMessage(lo))
		}
		if v.Hi != nil {
			hi, err := e.message(fd, v.Hi)
			if err != nil {
				return nil, err
			}
			r.Set(rangeMd.Fields().ByName("hi"), protoreflect.ValueOfMessage(hi))
		}
		r.Set(rangeMd.Fields().ByName("exclusive"), protoreflect.ValueOfBool(v.Exclusive))
		set("range", protoreflect.ValueOfMessage(r))
	case *object.Instance:
		data, err := e.reg.Marshal(v)
		if err != nil {
			return nil, err
		}
		objMd := fd.Messages().ByName("Object")
		o := dynamicpb.NewMessage(objMd)
		o.Set(objMd.Fields().ByName("class"), protoreflect.ValueOfString(v.Cls.QualifiedName()))
		o.Set(objMd.Fields().ByName("digest"), protoreflect.ValueOfString(v.Cls.Digest))
		o.Set(objMd.Fields().ByName("message"), protoreflect.ValueOfBytes(data))
		set("object", protoreflect.ValueOfMessage(o))
	default:
		if v == object.Nil {
			break
		}
		return nil, &NotSerialisable{Kind: kindName(v)}
	}
	return msg, nil
}

// NotSerialisable is the error for a value the envelope cannot carry: a
// proc, a class, a process.
type NotSerialisable struct{ Kind string }

func (e *NotSerialisable) Error() string { return "a " + e.Kind + " cannot be serialised" }

func (e *Encoder) items(fd protoreflect.FileDescriptor, name string, elems []object.Value) (*dynamicpb.Message, error) {
	md := fd.Messages().ByName(protoreflect.Name(name))
	m := dynamicpb.NewMessage(md)
	list := m.Mutable(md.Fields().ByName("items")).List()
	for _, el := range elems {
		em, err := e.message(fd, el)
		if err != nil {
			return nil, err
		}
		list.Append(protoreflect.ValueOfMessage(em))
	}
	return m, nil
}

// ClassResolver finds the class an Object message names, by qualified name
// and digest; nil when there is none.
type ClassResolver func(name, digest string) *object.Class

// Decoder reads values out of the envelope.
type Decoder struct {
	reg     *Registry
	resolve ClassResolver
}

// NewDecoder makes a decoder that finds classes through resolve.
func NewDecoder(reg *Registry, resolve ClassResolver) *Decoder {
	return &Decoder{reg: reg, resolve: resolve}
}

// Decode reads a value from its wire form.
func (d *Decoder) Decode(data []byte) (object.Value, error) {
	fd, err := envelope()
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(fd.Messages().ByName("Value"))
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return d.value(msg)
}

// Value reads an amber.Value message.
func (d *Decoder) Value(msg protoreflect.Message) (object.Value, error) { return d.value(msg) }

func (d *Decoder) value(msg protoreflect.Message) (object.Value, error) {
	md := msg.Descriptor()
	kind := msg.WhichOneof(md.Oneofs().ByName("kind"))
	if kind == nil {
		return object.Nil, nil
	}
	pv := msg.Get(kind)
	switch kind.Name() {
	case "bool_value":
		return object.Bool(pv.Bool()), nil
	case "int_value":
		return object.Int(pv.Int()), nil
	case "float_value":
		return object.Float(pv.Float()), nil
	case "string_value":
		return object.String(pv.String()), nil
	case "bytes_value":
		return object.Bytes(append([]byte{}, pv.Bytes()...)), nil
	case "list", "set":
		items, err := d.items(pv.Message())
		if err != nil {
			return nil, err
		}
		if kind.Name() == "set" {
			return object.NewSet(items...), nil
		}
		return object.NewList(items...), nil
	case "map":
		m := pv.Message()
		entries := m.Get(m.Descriptor().Fields().ByName("entries")).List()
		b := object.NewMapBuilder()
		for i := 0; i < entries.Len(); i++ {
			entry := entries.Get(i).Message()
			k, err := d.value(entry.Get(entry.Descriptor().Fields().ByName("key")).Message())
			if err != nil {
				return nil, err
			}
			v, err := d.value(entry.Get(entry.Descriptor().Fields().ByName("value")).Message())
			if err != nil {
				return nil, err
			}
			b.Set(k, v)
		}
		return b.Map(), nil
	case "range":
		r := pv.Message()
		fields := r.Descriptor().Fields()
		out := &object.Range{Exclusive: r.Get(fields.ByName("exclusive")).Bool()}
		if r.Has(fields.ByName("lo")) {
			lo, err := d.value(r.Get(fields.ByName("lo")).Message())
			if err != nil {
				return nil, err
			}
			out.Lo = lo
		}
		if r.Has(fields.ByName("hi")) {
			hi, err := d.value(r.Get(fields.ByName("hi")).Message())
			if err != nil {
				return nil, err
			}
			out.Hi = hi
		}
		return out, nil
	case "object":
		o := pv.Message()
		fields := o.Descriptor().Fields()
		name := o.Get(fields.ByName("class")).String()
		digest := o.Get(fields.ByName("digest")).String()
		cls := d.resolve(name, digest)
		if cls == nil {
			return nil, fmt.Errorf("no class %s (digest %s) is loaded", name, digest)
		}
		return d.reg.Unmarshal(cls, o.Get(fields.ByName("message")).Bytes())
	}
	return nil, fmt.Errorf("unknown envelope kind %s", kind.Name())
}

func (d *Decoder) items(m protoreflect.Message) ([]object.Value, error) {
	list := m.Get(m.Descriptor().Fields().ByName("items")).List()
	out := make([]object.Value, list.Len())
	for i := range out {
		v, err := d.value(list.Get(i).Message())
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func kindName(v object.Value) string {
	if v == object.Nil {
		return "nil"
	}
	return v.Class().QualifiedName()
}
