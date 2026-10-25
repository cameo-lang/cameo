package protogen

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/cameo-lang/cameo/src/object"
)

// An exception is a message like an instance: the fields its class
// declares, after the parts every exception has -- its message, the line
// it was raised on and its cause -- and those an exception of Ruby's core
// carries besides (object.Exception's Attrs). The parts take the numbers
// up to ExceptionNumbers, which a declared field cannot take.

// ExceptionNumbers is the highest field number an exception's own parts
// may take.
const ExceptionNumbers = 15

type partKind int

const (
	partString partKind = iota // a string
	partSymbol                 // a symbol, stored as a string
	partInt                    // an int64
	partValue                  // any value, as a cameo.Value
	partValues                 // a list of any values, as repeated cameo.Value
)

type exceptionPart struct {
	name   string
	number int32
	kind   partKind
}

// ownParts are the parts each class adds to its superclass's.
var ownParts = map[*object.Class][]exceptionPart{
	object.ExceptionClass:     {{"message", 1, partString}, {"line", 2, partInt}, {"cause", 3, partValue}},
	object.NameError:          {{"name", 4, partSymbol}, {"receiver", 5, partValue}},
	object.NoMethodError:      {{"args", 6, partValues}},
	object.KeyError:           {{"key", 4, partValue}, {"receiver", 5, partValue}},
	object.SystemExit:         {{"status", 4, partInt}},
	object.UncaughtThrowError: {{"tag", 4, partValue}, {"value", 5, partValue}},
}

// exceptionParts returns the parts of an exception class's message, the
// root's first; nil when cls is not an exception class.
func exceptionParts(cls *object.Class) []exceptionPart {
	if cls.IsModule || !cls.IsA(object.ExceptionClass) {
		return nil
	}
	var chain []*object.Class
	for c := cls; c != nil; c = c.Super {
		chain = append(chain, c)
	}
	var out []exceptionPart
	for i := len(chain) - 1; i >= 0; i-- {
		out = append(out, ownParts[chain[i]]...)
	}
	return out
}

// ExceptionPart reports whether name is one of the parts of an exception
// of class cls, which no field of the class may be named.
func ExceptionPart(cls *object.Class, name string) bool {
	for _, p := range exceptionParts(cls) {
		if p.name == name {
			return true
		}
	}
	return false
}

// partFields adds the parts of an exception class to its message.
func partFields(m *descriptorpb.DescriptorProto, parts []exceptionPart, oneofs *int) {
	for _, p := range parts {
		fp := &descriptorpb.FieldDescriptorProto{
			Name:     strPtr(p.name),
			JsonName: strPtr(jsonName(p.name)),
			Number:   int32Ptr(p.number),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		switch p.kind {
		case partString, partSymbol:
			fp.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		case partInt:
			fp.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		case partValue, partValues:
			fp.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fp.TypeName = strPtr(".cameo.Value")
		}
		if p.kind == partValues {
			fp.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		} else {
			optional(fp, m, oneofs)
		}
		m.Field = append(m.Field, fp)
	}
}

// partText is a part as a line of .proto text.
func partText(p exceptionPart) string {
	switch p.kind {
	case partString, partSymbol:
		return fmt.Sprintf("optional string %s = %d;", p.name, p.number)
	case partInt:
		return fmt.Sprintf("optional int64 %s = %d;", p.name, p.number)
	case partValues:
		return fmt.Sprintf("repeated cameo.Value %s = %d;", p.name, p.number)
	}
	return fmt.Sprintf("optional cameo.Value %s = %d;", p.name, p.number)
}

// ExceptionMessage builds the message an exception is. A part whose value
// cannot be serialised, such as a NameError's receiver when it is a class,
// is left unset.
func (r *Registry) ExceptionMessage(exc *object.Exception) (*dynamicpb.Message, error) {
	md, err := r.Message(exc.Cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	enc := NewEncoder(r)
	for _, p := range exceptionParts(exc.Cls) {
		fd := md.Fields().ByName(protoreflect.Name(p.name))
		var v object.Value
		switch p.name {
		case "message":
			msg.Set(fd, protoreflect.ValueOfString(exc.Message))
			continue
		case "line":
			if exc.Line != 0 {
				msg.Set(fd, protoreflect.ValueOfInt64(int64(exc.Line)))
			}
			continue
		case "cause":
			v = exc.Cause
		default:
			v = exc.Attrs[p.name]
		}
		if v.IsZero() {
			continue
		}
		switch p.kind {
		case partSymbol, partString:
			if s, ok := v.AsString(); ok {
				msg.Set(fd, protoreflect.ValueOfString(s))
			} else if s, ok := v.AsSymbol(); ok {
				msg.Set(fd, protoreflect.ValueOfString(s))
			}
		case partInt:
			if n, ok := v.AsInt(); ok {
				msg.Set(fd, protoreflect.ValueOfInt64(n))
			}
		case partValue:
			if vm, err := enc.Message(v); err == nil {
				msg.Set(fd, protoreflect.ValueOfMessage(vm))
			}
		case partValues:
			l, ok := v.AsList()
			if !ok {
				continue
			}
			list := msg.NewField(fd).List()
			for _, e := range l.Elems {
				em, err := enc.Message(e)
				if err != nil {
					list = nil
					break
				}
				list.Append(protoreflect.ValueOfMessage(em))
			}
			if list != nil {
				msg.Set(fd, protoreflect.ValueOfList(list))
			}
		}
	}
	if err := r.setFields(msg, md, exc.Cls, exc.Fields); err != nil {
		return nil, err
	}
	return msg, nil
}

// MarshalException is the wire form of an exception.
func (r *Registry) MarshalException(exc *object.Exception) ([]byte, error) {
	msg, err := r.ExceptionMessage(exc)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(msg)
}

// Exception reads an exception of class cls from its message, finding the
// classes of the values its parts hold as the decoder does.
func (d *Decoder) Exception(cls *object.Class, msg protoreflect.Message) (*object.Exception, error) {
	exc := &object.Exception{Cls: cls, Message: cls.QualifiedName()}
	md := msg.Descriptor()
	for _, p := range exceptionParts(cls) {
		fd := md.Fields().ByName(protoreflect.Name(p.name))
		if fd == nil || !msg.Has(fd) {
			continue
		}
		pv := msg.Get(fd)
		var v object.Value
		switch p.kind {
		case partString:
			v = object.String(pv.String())
		case partSymbol:
			v = object.Symbol(pv.String())
		case partInt:
			v = object.Int(pv.Int())
		case partValue:
			// A value of a class this side does not have, such as the
			// top-level self of another interpreter, is left out, as one
			// the envelope cannot carry was.
			x, err := d.value(pv.Message())
			if err != nil {
				continue
			}
			v = x
		case partValues:
			list := pv.List()
			elems := make([]object.Value, list.Len())
			var err error
			for i := range elems {
				if elems[i], err = d.value(list.Get(i).Message()); err != nil {
					break
				}
			}
			if err != nil {
				continue
			}
			v = object.NewList(elems...)
		}
		switch p.name {
		case "message":
			exc.Message = pv.String()
		case "line":
			exc.Line = int(pv.Int())
		case "cause":
			exc.Cause = v
		default:
			if exc.Attrs == nil {
				exc.Attrs = map[string]object.Value{}
			}
			exc.Attrs[p.name] = v
		}
	}
	fields, err := d.reg.readFields(cls, msg)
	if err != nil {
		return nil, err
	}
	if len(cls.Fields) > 0 {
		exc.Fields = fields
	}
	return exc, nil
}

// UnmarshalException reads the wire form of an exception of class cls.
func (d *Decoder) UnmarshalException(cls *object.Class, data []byte) (*object.Exception, error) {
	md, err := d.reg.Message(cls)
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return d.Exception(cls, msg)
}
