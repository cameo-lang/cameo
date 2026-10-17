package protogen

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/radiospiel/amber/src/object"
)

// ToMessage builds the message an instance is. The instance keeps it, since
// its fields never change.
func (r *Registry) ToMessage(inst *object.Instance) (*dynamicpb.Message, error) {
	if m, ok := inst.Message.(*dynamicpb.Message); ok {
		return m, nil
	}
	md, err := r.Message(inst.Cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	for i, f := range inst.Cls.Fields {
		v := inst.Fields[i]
		if v == nil || v == object.Nil {
			continue
		}
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		pv, err := r.toProto(fd, f.Type, v, msg)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", inst.Cls.QualifiedName(), f.Name, err)
		}
		msg.Set(fd, pv)
	}
	inst.Message = msg
	return msg, nil
}

func (r *Registry) toProto(fd protoreflect.FieldDescriptor, t *object.FieldType, v object.Value, parent *dynamicpb.Message) (protoreflect.Value, error) {
	switch t.Kind {
	case "list", "set":
		list := parent.NewField(fd).List()
		var elems []object.Value
		switch v := v.(type) {
		case *object.List:
			elems = v.Elems
		case *object.Set:
			elems = v.Elems()
		}
		for _, e := range elems {
			pv, err := r.scalar(fd, t.Elem, e)
			if err != nil {
				return protoreflect.Value{}, err
			}
			list.Append(pv)
		}
		return protoreflect.ValueOfList(list), nil
	case "map":
		m := parent.NewField(fd).Map()
		src := v.(*object.Map)
		for i, k := range src.Keys() {
			key, err := r.scalar(fd.MapKey(), &object.FieldType{Kind: t.Key}, k)
			if err != nil {
				return protoreflect.Value{}, err
			}
			val, err := r.scalar(fd.MapValue(), t.Elem, src.Values()[i])
			if err != nil {
				return protoreflect.Value{}, err
			}
			m.Set(key.MapKey(), val)
		}
		return protoreflect.ValueOfMap(m), nil
	}
	return r.scalar(fd, t, v)
}

// scalar converts one non-repeated value.
func (r *Registry) scalar(fd protoreflect.FieldDescriptor, t *object.FieldType, v object.Value) (protoreflect.Value, error) {
	switch t.Kind {
	case "string":
		return protoreflect.ValueOfString(string(v.(object.String))), nil
	case "int":
		return protoreflect.ValueOfInt64(int64(v.(object.Int))), nil
	case "float":
		switch x := v.(type) {
		case object.Float:
			return protoreflect.ValueOfFloat64(float64(x)), nil
		case object.Int:
			return protoreflect.ValueOfFloat64(float64(x)), nil
		}
	case "bool":
		return protoreflect.ValueOfBool(bool(v.(object.Bool))), nil
	case "bytes":
		return protoreflect.ValueOfBytes([]byte(v.(object.Bytes))), nil
	case "enum":
		name := string(v.(object.Symbol))
		for i, e := range t.Enum {
			if e == name {
				return protoreflect.ValueOfEnum(protoreflect.EnumNumber(i + 1)), nil
			}
		}
		return protoreflect.Value{}, fmt.Errorf("%s is not among the enum's values", name)
	case "ref":
		inst, ok := v.(*object.Instance)
		if !ok {
			break
		}
		msg, err := r.ToMessage(inst)
		if err != nil {
			return protoreflect.Value{}, err
		}
		if inst.Cls != t.Ref {
			// A subclass instance is stored as its parent's message: the
			// fields the parent declares, which the subclass shares.
			msg, err = r.narrow(msg, t.Ref)
			if err != nil {
				return protoreflect.Value{}, err
			}
		}
		return protoreflect.ValueOfMessage(msg), nil
	}
	return protoreflect.Value{}, fmt.Errorf("cannot store %T in a %s field", v, t.Kind)
}

// narrow re-reads a subclass message as its parent's message.
func (r *Registry) narrow(msg *dynamicpb.Message, parent *object.Class) (*dynamicpb.Message, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	md, err := r.Message(parent)
	if err != nil {
		return nil, err
	}
	out := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// FromMessage builds an instance of cls from its message.
func (r *Registry) FromMessage(cls *object.Class, msg protoreflect.Message) (*object.Instance, error) {
	inst := &object.Instance{Cls: cls, Fields: make([]object.Value, len(cls.Fields))}
	md := msg.Descriptor()
	for i, f := range cls.Fields {
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		if fd == nil {
			continue
		}
		if !msg.Has(fd) && (f.Type.Kind != "list" && f.Type.Kind != "set" && f.Type.Kind != "map") {
			continue
		}
		v, err := r.fromProto(fd, f.Type, msg.Get(fd))
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", cls.QualifiedName(), f.Name, err)
		}
		inst.Fields[i] = v
	}
	if dm, ok := msg.(*dynamicpb.Message); ok {
		inst.Message = dm
	}
	return inst, nil
}

func (r *Registry) fromProto(fd protoreflect.FieldDescriptor, t *object.FieldType, pv protoreflect.Value) (object.Value, error) {
	switch t.Kind {
	case "list", "set":
		list := pv.List()
		elems := make([]object.Value, list.Len())
		for i := range elems {
			v, err := r.fromScalar(t.Elem, list.Get(i))
			if err != nil {
				return nil, err
			}
			elems[i] = v
		}
		if t.Kind == "set" {
			return object.NewSet(elems...), nil
		}
		return object.NewList(elems...), nil
	case "map":
		// A protobuf map has no order; the entries come back sorted by key
		// so a decoded value prints the same every time.
		type entry struct{ key, val object.Value }
		var entries []entry
		var err error
		pv.Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			key, e := r.fromScalar(&object.FieldType{Kind: t.Key}, k.Value())
			if e != nil {
				err = e
				return false
			}
			val, e := r.fromScalar(t.Elem, v)
			if e != nil {
				err = e
				return false
			}
			entries = append(entries, entry{key, val})
			return true
		})
		if err != nil {
			return nil, err
		}
		sort.Slice(entries, func(i, j int) bool { return object.Key(entries[i].key) < object.Key(entries[j].key) })
		b := object.NewMapBuilder()
		for _, e := range entries {
			b.Set(e.key, e.val)
		}
		return b.Map(), nil
	}
	return r.fromScalar(t, pv)
}

func (r *Registry) fromScalar(t *object.FieldType, pv protoreflect.Value) (object.Value, error) {
	switch t.Kind {
	case "string":
		return object.String(pv.String()), nil
	case "int":
		return object.IntValue(pv.Int()), nil
	case "float":
		return object.Float(pv.Float()), nil
	case "bool":
		return object.Bool(pv.Bool()), nil
	case "bytes":
		return object.Bytes(append([]byte{}, pv.Bytes()...)), nil
	case "enum":
		n := int(pv.Enum())
		if n == 0 {
			return object.Nil, nil
		}
		if n > len(t.Enum) {
			return nil, fmt.Errorf("enum value %d out of range", n)
		}
		return object.Symbol(t.Enum[n-1]), nil
	case "ref":
		return r.FromMessage(t.Ref, pv.Message())
	}
	return nil, fmt.Errorf("cannot read a %s field", t.Kind)
}

// Marshal is the wire form of an instance.
func (r *Registry) Marshal(inst *object.Instance) ([]byte, error) {
	msg, err := r.ToMessage(inst)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Unmarshal reads the wire form of cls's message into an instance.
func (r *Registry) Unmarshal(cls *object.Class, data []byte) (*object.Instance, error) {
	md, err := r.Message(cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return r.FromMessage(cls, msg)
}
