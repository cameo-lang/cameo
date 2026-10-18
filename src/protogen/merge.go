package protogen

// Programs that share a store of messages may each have a definition of a
// message of their own: an older one lacks a field a newer one added, and
// two may each add a field the other lacks. Merge makes one definition
// that reads what both write. It matches fields by name, as protobuf JSON
// does:
//
//   - A field both have keeps the registered definition's number and type.
//     The other definition must give it the same type -- the same scalar
//     kind, the same enum with the same values, or the same message, which
//     may lack fields -- the same cardinality and the same oneof.
//   - A field only the registered definition has stays.
//   - A field only the other definition has is added, numbered by
//     FieldNumber from its name, with the enums and messages it uses and
//     the imports they need. It may not be in a oneof, nor have a name the
//     registered definition reserves.
//
// So the merged message reads the protobuf JSON of both, and the wire form
// of both where their numbers agree: always for numbers derived from the
// names, but for a hash collision.

import (
	"errors"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Merge returns the file of registered, a top-level message, with the
// fields of next, a definition of the same message, that registered lacks
// (see above). It returns registered's file unchanged when next only lacks
// fields, and fails when next changes a field registered has.
func Merge(registered, next protoreflect.MessageDescriptor) (*descriptorpb.FileDescriptorProto, error) {
	if registered.FullName() != next.FullName() {
		return nil, fmt.Errorf("%s and %s are different messages", registered.FullName(), next.FullName())
	}
	if registered.Parent() != registered.ParentFile() {
		return nil, fmt.Errorf("%s: Merge takes a top-level message", registered.FullName())
	}
	file := protodesc.ToFileDescriptorProto(registered.ParentFile())
	file.SourceCodeInfo = nil
	m := &merger{
		file:  file,
		msg:   file.MessageType[slices.IndexFunc(file.MessageType, func(d *descriptorpb.DescriptorProto) bool { return d.GetName() == string(registered.Name()) })],
		top:   next,
		added: map[int64]bool{},
	}
	taken := func(n int64) bool {
		return m.added[n] || registered.Fields().ByNumber(protoreflect.FieldNumber(n)) != nil ||
			registered.ReservedRanges().Has(protoreflect.FieldNumber(n))
	}
	for i := 0; i < next.Fields().Len(); i++ {
		f := next.Fields().Get(i)
		at := string(next.Name()) + "." + string(f.Name())
		if have := registered.Fields().ByName(f.Name()); have != nil {
			if err := sameType(have, f, at); err != nil {
				return nil, err
			}
			continue
		}
		if f.ContainingOneof() != nil && !f.ContainingOneof().IsSynthetic() {
			return nil, fmt.Errorf("%s: Merge does not add a field in a oneof", at)
		}
		if registered.ReservedNames().Has(f.Name()) {
			return nil, fmt.Errorf("%s: the name is reserved", at)
		}
		if err := m.uses(f); err != nil {
			return nil, fmt.Errorf("%s: %w", at, err)
		}
		n := FieldNumber(string(f.Name()), taken)
		m.added[n] = true
		fp := protodesc.ToFieldDescriptorProto(f)
		fp.Number = proto.Int32(int32(n))
		fp.OneofIndex = nil
		if f.HasOptionalKeyword() {
			fp.OneofIndex = proto.Int32(int32(len(m.msg.OneofDecl)))
			m.msg.OneofDecl = append(m.msg.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: proto.String("_" + string(f.Name()))})
		}
		m.msg.Field = append(m.msg.Field, fp)
	}
	if _, err := protodesc.NewFile(file, resolver(registered.ParentFile(), next.ParentFile())); err != nil {
		return nil, fmt.Errorf("the merged %s: %w", registered.FullName(), err)
	}
	return file, nil
}

// merger collects what Merge adds to the registered file.
type merger struct {
	file *descriptorpb.FileDescriptorProto
	// msg is the registered message in file, and top the other definition.
	msg   *descriptorpb.DescriptorProto
	top   protoreflect.MessageDescriptor
	added map[int64]bool
}

// uses adds the declarations and imports f's type needs.
func (m *merger) uses(f protoreflect.FieldDescriptor) error {
	if f.IsMap() {
		if err := m.declare(f.Message()); err != nil {
			return err
		}
		f = f.MapValue()
	}
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return m.declare(f.Message())
	case protoreflect.EnumKind:
		return m.declare(f.Enum())
	}
	return nil
}

// declare adds d, a message or an enum, at the top of the file or in the
// registered message, as next declares it, and the types it uses; unless
// the registered file has it, the same. A type of another file adds an
// import.
func (m *merger) declare(d protoreflect.Descriptor) error {
	if d.FullName() == m.top.FullName() {
		return nil
	}
	if path := d.ParentFile().Path(); path != m.top.ParentFile().Path() {
		if path != m.file.GetName() && !slices.Contains(m.file.Dependency, path) {
			m.file.Dependency = append(m.file.Dependency, path)
		}
		return nil
	}
	var messages *[]*descriptorpb.DescriptorProto
	var enums *[]*descriptorpb.EnumDescriptorProto
	switch d.Parent() {
	case m.top.ParentFile():
		messages, enums = &m.file.MessageType, &m.file.EnumType
	case m.top:
		messages, enums = &m.msg.NestedType, &m.msg.EnumType
	default:
		return fmt.Errorf("%s: Merge adds only types declared at the top of the file or in the message", d.FullName())
	}
	switch d := d.(type) {
	case protoreflect.MessageDescriptor:
		dp := protodesc.ToDescriptorProto(d)
		if i := slices.IndexFunc(*messages, func(h *descriptorpb.DescriptorProto) bool { return h.GetName() == dp.GetName() }); i >= 0 {
			if !proto.Equal((*messages)[i], dp) {
				return fmt.Errorf("%s is declared otherwise in the registered definition", d.FullName())
			}
			return nil
		}
		*messages = append(*messages, dp)
		for i := 0; i < d.Fields().Len(); i++ {
			if err := m.uses(d.Fields().Get(i)); err != nil {
				return err
			}
		}
	case protoreflect.EnumDescriptor:
		ep := protodesc.ToEnumDescriptorProto(d)
		if i := slices.IndexFunc(*enums, func(h *descriptorpb.EnumDescriptorProto) bool { return h.GetName() == ep.GetName() }); i >= 0 {
			if !proto.Equal((*enums)[i], ep) {
				return fmt.Errorf("%s is declared otherwise in the registered definition", d.FullName())
			}
			return nil
		}
		*enums = append(*enums, ep)
	}
	return nil
}

// sameType returns why next, a field of the other definition, does not
// have the type of have, the registered field of its name.
func sameType(have, next protoreflect.FieldDescriptor, at string) error {
	switch {
	case have.IsMap() != next.IsMap() || have.IsList() != next.IsList():
		return fmt.Errorf("%s: a field keeps its cardinality: singular, repeated or map", at)
	case oneofName(have) != oneofName(next):
		return fmt.Errorf("%s: moves from oneof %q to oneof %q", at, oneofName(have), oneofName(next))
	case have.IsMap():
		if err := sameType(have.MapKey(), next.MapKey(), at+" (key)"); err != nil {
			return err
		}
		return sameType(have.MapValue(), next.MapValue(), at+" (value)")
	case have.Kind() != next.Kind():
		return fmt.Errorf("%s: %s cannot become %s", at, have.Kind(), next.Kind())
	case have.Kind() == protoreflect.EnumKind:
		if have.Enum().FullName() != next.Enum().FullName() ||
			!proto.Equal(protodesc.ToEnumDescriptorProto(have.Enum()), protodesc.ToEnumDescriptorProto(next.Enum())) {
			return fmt.Errorf("%s: the enum %s differs from the registered one", at, next.Enum().FullName())
		}
	case have.Kind() == protoreflect.MessageKind || have.Kind() == protoreflect.GroupKind:
		return lacksFields(have.Message(), next.Message(), at)
	}
	return nil
}

// lacksFields returns why next, a message, is not have with fields left
// out: each of its fields have's under the same name and number, of the
// same type.
func lacksFields(have, next protoreflect.MessageDescriptor, at string) error {
	if have.FullName() != next.FullName() {
		return fmt.Errorf("%s: %s cannot become %s", at, have.FullName(), next.FullName())
	}
	for i := 0; i < next.Fields().Len(); i++ {
		f := next.Fields().Get(i)
		h := have.Fields().ByName(f.Name())
		if h == nil || h.Number() != f.Number() {
			return fmt.Errorf("%s: %s.%s = %d is not in the registered definition", at, next.Name(), f.Name(), f.Number())
		}
		if h.Message() != nil && h.Message().FullName() == have.FullName() {
			// A message that holds itself.
			continue
		}
		if err := sameType(h, f, at+"."+string(f.Name())); err != nil {
			return err
		}
	}
	return nil
}

func oneofName(f protoreflect.FieldDescriptor) protoreflect.Name {
	if o := f.ContainingOneof(); o != nil && !o.IsSynthetic() {
		return o.Name()
	}
	return ""
}

// resolver finds the files files import, and theirs.
func resolver(files ...protoreflect.FileDescriptor) *protoregistry.Files {
	reg := &protoregistry.Files{}
	var add func(protoreflect.FileDescriptor)
	add = func(f protoreflect.FileDescriptor) {
		for i := 0; i < f.Imports().Len(); i++ {
			imp := f.Imports().Get(i).FileDescriptor
			if _, err := reg.FindFileByPath(imp.Path()); errors.Is(err, protoregistry.NotFound) {
				add(imp)
				_ = reg.RegisterFile(imp)
			}
		}
	}
	for _, f := range files {
		add(f)
	}
	return reg
}
