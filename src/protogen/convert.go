package protogen

import (
	"fmt"
	"github.com/cameo-lang/cameo/src/str"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/object"
)

// ToMessage builds the message an instance is. The instance keeps it, since
// its fields never change.
func (r *Registry) ToMessage(inst *object.Instance) (*dynamicpb.Message, error) {
	return r.toMessage(inst, nil)
}

// toMessage is ToMessage with the encoder the message is built for, which
// encodes the procs of proc fields; nil for a caller that has none, where
// an instance with a proc cannot be a message. An instance of a class
// with a proc field does not keep its message: a closure is encoded for
// its reader, with the code or without, in a dump with its frame.
func (r *Registry) toMessage(inst *object.Instance, e *Encoder) (*dynamicpb.Message, error) {
	kept := !HasProc(inst.Cls)
	if m, ok := inst.Message.(*dynamicpb.Message); ok && kept {
		return m, nil
	}
	md, err := r.Message(inst.Cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := r.setFields(msg, md, inst.Cls, inst.Fields, e); err != nil {
		return nil, err
	}
	if kept {
		inst.Message = msg
	}
	return msg, nil
}

// setFields sets the fields cls declares, fields in its order, on msg.
func (r *Registry) setFields(msg *dynamicpb.Message, md protoreflect.MessageDescriptor, cls *object.Class, fields []object.Value, e *Encoder) error {
	for i, f := range cls.Fields {
		if i >= len(fields) {
			break
		}
		v := fields[i]
		if v.IsZero() || v.IsNil() {
			continue
		}
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		pv, err := r.toProto(fd, f.Type, v, msg, e)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", cls.QualifiedName(), f.Name, err)
		}
		msg.Set(fd, pv)
	}
	return nil
}

func (r *Registry) toProto(fd protoreflect.FieldDescriptor, t *object.FieldType, v object.Value, parent *dynamicpb.Message, enc *Encoder) (protoreflect.Value, error) {
	switch t.Kind {
	case "list", "set":
		list := parent.NewField(fd).List()
		var elems []object.Value
		switch v.Kind() {
		case object.KindList:
			v := v.List()
			elems = v.Elems
		case object.KindSet:
			v := v.Set()
			elems = v.Elems()
		}
		for _, e := range elems {
			pv, err := r.scalar(fd, t.Elem, e, enc)
			if err != nil {
				return protoreflect.Value{}, err
			}
			list.Append(pv)
		}
		return protoreflect.ValueOfList(list), nil
	case "map":
		m := parent.NewField(fd).Map()
		src := v.Map()
		entries := make([]mapEntry, len(src.Keys()))
		for i, k := range src.Keys() {
			key, err := r.scalar(fd.MapKey(), object.ScalarField(t.Key), k, enc)
			if err != nil {
				return protoreflect.Value{}, err
			}
			entries[i] = mapEntry{key: key.MapKey(), value: src.Values()[i]}
		}
		if holdsProc(t.Elem) {
			sortEntries(entries)
		}
		for _, e := range entries {
			val, err := r.scalar(fd.MapValue(), t.Elem, e.value, enc)
			if err != nil {
				return protoreflect.Value{}, err
			}
			m.Set(e.key, val)
		}
		return protoreflect.ValueOfMap(m), nil
	}
	return r.scalar(fd, t, v, enc)
}

// scalar converts one non-repeated value. enc encodes a proc, and the
// procs of an instance the value refers to.
func (r *Registry) scalar(fd protoreflect.FieldDescriptor, t *object.FieldType, v object.Value, enc *Encoder) (protoreflect.Value, error) {
	if s := t.Scalar; s != nil {
		return scalarToProto(s, v), nil
	}
	switch t.Kind {
	case "proc":
		// A proc field is a cameo.Closure, which the encoder's codec
		// makes of the proc.
		if enc == nil || enc.procs == nil {
			return protoreflect.Value{}, &NotSerialisable{Kind: "a Proc", Class: "Proc"}
		}
		c, err := enc.procs.EncodeProc(enc, v)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(c.ProtoReflect()), nil
	case "enum":
		name := string(v.Str())
		for i, e := range t.Enum {
			if e == name {
				return protoreflect.ValueOfEnum(protoreflect.EnumNumber(i + 1)), nil
			}
		}
		return protoreflect.Value{}, fmt.Errorf("%s is not among the enum's values", name)
	case "ref":
		inst, ok := v.AsInstance()
		if !ok {
			break
		}
		if inst.Cls != t.Ref && HasProc(inst.Cls) {
			// The parent's message is made of the parent's fields alone,
			// so that no proc is encoded that the reader does not read:
			// the first closure of a unit carries its code.
			msg, err := r.parentMessage(inst, t.Ref, enc)
			if err != nil {
				return protoreflect.Value{}, err
			}
			return protoreflect.ValueOfMessage(msg), nil
		}
		msg, err := r.toMessage(inst, enc)
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

// scalarToProto converts a value the field checked against s.
func scalarToProto(s *object.ScalarType, v object.Value) protoreflect.Value {
	switch s.Kind {
	case object.ScalarString:
		return protoreflect.ValueOfString(v.Str())
	case object.ScalarBytes:
		return protoreflect.ValueOfBytes(v.Bytes())
	case object.ScalarBool:
		return protoreflect.ValueOfBool(v.Bool())
	case object.ScalarFloat:
		x, _ := v.AsNumber()
		if s.Bits == 32 {
			return protoreflect.ValueOfFloat32(float32(x))
		}
		return protoreflect.ValueOfFloat64(x)
	}
	switch {
	case s.Bits == 0:
		return protoreflect.ValueOfString(object.IntegerDigits(v))
	case s.Unsigned && s.Bits == 32:
		return protoreflect.ValueOfUint32(uint32(v.Int()))
	case s.Unsigned:
		return protoreflect.ValueOfUint64(object.IntegerUint64(v))
	case s.Bits == 32:
		return protoreflect.ValueOfInt32(int32(v.Int()))
	}
	return protoreflect.ValueOfInt64(v.Int())
}

// scalarFromProto reads a protobuf value of the scalar type s.
func scalarFromProto(s *object.ScalarType, pv protoreflect.Value) (object.Value, error) {
	switch s.Kind {
	case object.ScalarString:
		return object.NewString(pv.String(), str.UTF8), nil
	case object.ScalarBytes:
		return object.Bytes(append([]byte{}, pv.Bytes()...)), nil
	case object.ScalarBool:
		return object.Bool(pv.Bool()), nil
	case object.ScalarFloat:
		return object.Float(pv.Float()), nil
	}
	switch {
	case s.Bits == 0:
		v, ok := object.ParseInteger(pv.String())
		if !ok {
			return object.Value{}, fmt.Errorf("a bignum field holds %q, which is not an integer", pv.String())
		}
		return v, nil
	case s.Unsigned:
		return object.Uint64Integer(pv.Uint()), nil
	}
	return object.Int(pv.Int()), nil
}

// parentMessage builds the message of parent from an instance of a class
// that inherits from it: the fields parent declares, by name.
func (r *Registry) parentMessage(inst *object.Instance, parent *object.Class, e *Encoder) (*dynamicpb.Message, error) {
	md, err := r.Message(parent)
	if err != nil {
		return nil, err
	}
	fields := make([]object.Value, len(parent.Fields))
	for i, pf := range parent.Fields {
		for j, f := range inst.Cls.Fields {
			if f.Name == pf.Name && j < len(inst.Fields) {
				fields[i] = inst.Fields[j]
			}
		}
	}
	msg := dynamicpb.NewMessage(md)
	if err := r.setFields(msg, md, parent, fields, e); err != nil {
		return nil, err
	}
	return msg, nil
}

// mapEntry is one entry of a map field: its key as protobuf has it, and
// its value before or after the conversion.
type mapEntry struct {
	key   protoreflect.MapKey
	value object.Value
	proto protoreflect.Value
}

// sortEntries puts the entries of a map field in the order of their keys.
// A protobuf map has no order, and the closures in one are written and
// read in an order both sides agree on: the first closure of a unit
// carries its code, which the ones after it need.
func sortEntries(entries []mapEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		switch a := entries[i].key.Interface().(type) {
		case bool:
			return !a && entries[j].key.Bool()
		case string:
			return a < entries[j].key.String()
		case uint32, uint64:
			return entries[i].key.Uint() < entries[j].key.Uint()
		}
		return entries[i].key.Int() < entries[j].key.Int()
	})
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

// fromMessage builds an instance of cls from its message. d is the
// decoder the message is read by, which decodes the closures of proc
// fields; nil for a caller that has none, where a message with a closure
// cannot be read.
func (r *Registry) fromMessage(cls *object.Class, msg protoreflect.Message, d *Decoder) (*object.Instance, error) {
	fields, err := r.readFields(cls, msg, d)
	if err != nil {
		return nil, err
	}
	inst := &object.Instance{Cls: cls, Fields: fields}
	if dm, ok := msg.(*dynamicpb.Message); ok && !HasProc(cls) {
		inst.Message = dm
	}
	return inst, nil
}

// readFields reads the fields cls declares from msg, in the class's order.
func (r *Registry) readFields(cls *object.Class, msg protoreflect.Message, d *Decoder) ([]object.Value, error) {
	fields := make([]object.Value, len(cls.Fields))
	md := msg.Descriptor()
	for i, f := range cls.Fields {
		fd := md.Fields().ByName(protoreflect.Name(f.Name))
		if fd == nil {
			continue
		}
		// A field with presence that is not set is nil; one without
		// presence, as a native class's scalars may be, is its zero value.
		if !msg.Has(fd) && fd.HasPresence() {
			continue
		}
		v, err := r.fromProto(fd, f.Type, msg.Get(fd), d)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", cls.QualifiedName(), f.Name, err)
		}
		fields[i] = v
	}
	return fields, nil
}

func (r *Registry) fromProto(fd protoreflect.FieldDescriptor, t *object.FieldType, pv protoreflect.Value, d *Decoder) (object.Value, error) {
	switch t.Kind {
	case "list", "set":
		list := pv.List()
		elems := make([]object.Value, list.Len())
		for i := range elems {
			v, err := r.fromScalar(t.Elem, list.Get(i), d)
			if err != nil {
				return object.Value{}, err
			}
			elems[i] = v
		}
		if t.Kind == "set" {
			return object.SetValue(object.NewSet(elems...)), nil
		}
		return object.NewList(elems...), nil
	case "map":
		// A protobuf map has no order; the entries come back sorted by key
		// so a decoded value prints the same every time.
		type entry struct{ key, val object.Value }
		var read []mapEntry
		pv.Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			read = append(read, mapEntry{key: k, proto: v})
			return true
		})
		if holdsProc(t.Elem) {
			// The order the closures were written in (toProto).
			sortEntries(read)
		}
		entries := make([]entry, len(read))
		for i, e := range read {
			key, err := r.fromScalar(object.ScalarField(t.Key), e.key.Value(), d)
			if err != nil {
				return object.Value{}, err
			}
			val, err := r.fromScalar(t.Elem, e.proto, d)
			if err != nil {
				return object.Value{}, err
			}
			entries[i] = entry{key, val}
		}
		sort.Slice(entries, func(i, j int) bool { return object.Key(entries[i].key) < object.Key(entries[j].key) })
		b := object.NewMapBuilder()
		for _, e := range entries {
			b.Set(e.key, e.val)
		}
		return object.MapValue(b.Map()), nil
	}
	return r.fromScalar(t, pv, d)
}

func (r *Registry) fromScalar(t *object.FieldType, pv protoreflect.Value, d *Decoder) (object.Value, error) {
	if s := t.Scalar; s != nil {
		return scalarFromProto(s, pv)
	}
	switch t.Kind {
	case "proc":
		// A proc field is a cameo.Closure, which the decoder's codec
		// makes a proc of.
		if d == nil || d.procs == nil {
			return object.Value{}, &NotSerialisable{Kind: "a Proc", Class: "Proc"}
		}
		data, err := proto.Marshal(pv.Message().Interface())
		if err != nil {
			return object.Value{}, err
		}
		c := &cameopb.Closure{}
		if err := proto.Unmarshal(data, c); err != nil {
			return object.Value{}, err
		}
		return d.procs.DecodeProc(d, c)
	case "enum":
		n := int(pv.Enum())
		if n == 0 {
			return object.Nil, nil
		}
		if n > len(t.Enum) {
			return object.Value{}, fmt.Errorf("enum value %d out of range", n)
		}
		return object.Symbol(t.Enum[n-1]), nil
	case "ref":
		inst, err := r.fromMessage(t.Ref, pv.Message(), d)
		if err != nil {
			return object.Value{}, err
		}
		return object.InstanceValue(inst), nil
	}
	return object.Value{}, fmt.Errorf("cannot read a %s field", t.Kind)
}

// Marshal is the wire form of an instance.
func (r *Registry) Marshal(inst *object.Instance) ([]byte, error) {
	return r.marshal(inst, nil)
}

// marshal is Marshal for the encoder e, which encodes the procs of proc
// fields.
func (r *Registry) marshal(inst *object.Instance, e *Encoder) ([]byte, error) {
	msg, err := r.toMessage(inst, e)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Instance is the wire form of an instance: its class's message, the
// procs of its proc fields as closures through the encoder's codec.
func (e *Encoder) Instance(inst *object.Instance) ([]byte, error) {
	return e.reg.marshal(inst, e)
}

// Unmarshal reads the wire form of cls's message into an instance.
func (r *Registry) Unmarshal(cls *object.Class, data []byte) (*object.Instance, error) {
	return r.unmarshal(cls, data, nil)
}

// Instance reads the wire form of cls's message into an instance, the
// closures of its proc fields through the decoder's codec.
func (d *Decoder) Instance(cls *object.Class, data []byte) (*object.Instance, error) {
	return d.reg.unmarshal(cls, data, d)
}

// InstanceOf builds an instance of cls from its message, as Instance
// does from the wire form.
func (d *Decoder) InstanceOf(cls *object.Class, msg protoreflect.Message) (*object.Instance, error) {
	return d.reg.fromMessage(cls, msg, d)
}

// unmarshal is Unmarshal for the decoder d, which decodes the closures
// of proc fields.
func (r *Registry) unmarshal(cls *object.Class, data []byte, d *Decoder) (*object.Instance, error) {
	md, err := r.Message(cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return r.fromMessage(cls, msg, d)
}
