// Package protogen is cameo's protobuf side: the message descriptor each
// class is, the .proto text that describes it, the conversion between an
// instance and its message, and the cameo.Value envelope that carries any
// value across a boundary. The design is in doc/design/cameo.md.
package protogen

import (
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/cameo-lang/cameo/src/object"
)

// Registry builds and keeps the descriptors of the classes one interpreter
// loads. A class is a value, so a rebinding of a name is another class with
// a descriptor of its own.
type Registry struct {
	mu    sync.Mutex
	files map[*object.Class]protoreflect.FileDescriptor
}

// NewRegistry makes an empty registry.
func NewRegistry() *Registry {
	return &Registry{files: map[*object.Class]protoreflect.FileDescriptor{}}
}

// Package is the protobuf package of a class: cameo for a top-level class,
// the lexical modules lower-cased and joined with dots otherwise.
func Package(cls *object.Class) string {
	var parts []string
	for m := cls.Lexical; m != nil; m = m.Lexical {
		parts = append([]string{strings.ToLower(m.Name)}, parts...)
	}
	if len(parts) == 0 {
		return "cameo"
	}
	return strings.Join(parts, ".")
}

// FileName is the .proto file a class is described in.
func FileName(cls *object.Class) string {
	return strings.ReplaceAll(Package(cls), ".", "/") + "/" + cls.Name + ".proto"
}

// Message returns the descriptor of the message a class is.
func (r *Registry) Message(cls *object.Class) (protoreflect.MessageDescriptor, error) {
	if n := nativeOf(cls); n != nil {
		return n.md, nil
	}
	fd, err := r.file(cls)
	if err != nil {
		return nil, err
	}
	return fd.Messages().ByName(protoreflect.Name(cls.Name)), nil
}

func (r *Registry) file(cls *object.Class) (protoreflect.FileDescriptor, error) {
	if n := nativeOf(cls); n != nil {
		return n.md.ParentFile(), nil
	}
	r.mu.Lock()
	fd, ok := r.files[cls]
	r.mu.Unlock()
	if ok {
		return fd, nil
	}
	exception := exceptionParts(cls) != nil
	if cls.Digest == "" && !exception || cls.IsModule {
		return nil, fmt.Errorf("%s is not a class with fields", cls.QualifiedName())
	}
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    strPtr(FileName(cls)),
		Package: strPtr(Package(cls)),
		Syntax:  strPtr("proto3"),
	}
	resolver := &chain{}
	if exception {
		// An exception's cause, and the values Ruby's exceptions carry, are
		// cameo.Values.
		env, err := envelope()
		if err != nil {
			return nil, err
		}
		fdp.Dependency = append(fdp.Dependency, env.Path())
		resolver.add(env)
	}
	for _, dep := range refs(cls) {
		if dep == cls {
			continue
		}
		depFile, err := r.file(dep)
		if err != nil {
			return nil, err
		}
		fdp.Dependency = append(fdp.Dependency, depFile.Path())
		resolver.add(depFile)
	}
	fdp.MessageType = []*descriptorpb.DescriptorProto{messageProto(cls)}
	fd, err := protodesc.NewFile(fdp, resolver)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cls.QualifiedName(), err)
	}
	r.mu.Lock()
	r.files[cls] = fd
	r.mu.Unlock()
	return fd, nil
}

// refs lists the classes a class's fields refer to, each once.
func refs(cls *object.Class) []*object.Class {
	var out []*object.Class
	seen := map[*object.Class]bool{}
	var walk func(t *object.FieldType)
	walk = func(t *object.FieldType) {
		if t == nil {
			return
		}
		if t.Ref != nil && !seen[t.Ref] {
			seen[t.Ref] = true
			out = append(out, t.Ref)
		}
		walk(t.Elem)
	}
	for _, f := range cls.Fields {
		walk(f.Type)
	}
	return out
}

// messageProto builds the DescriptorProto of a class: one field per
// declared field, a nested enum per enum field, a map entry message per
// map field, and the reserved numbers and names.
func messageProto(cls *object.Class) *descriptorpb.DescriptorProto {
	m := &descriptorpb.DescriptorProto{Name: strPtr(cls.Name)}
	for _, n := range cls.ReservedNumbers {
		m.ReservedRange = append(m.ReservedRange, &descriptorpb.DescriptorProto_ReservedRange{Start: int32Ptr(int32(n)), End: int32Ptr(int32(n + 1))})
	}
	m.ReservedName = append(m.ReservedName, cls.ReservedNames...)
	oneofs := 0
	partFields(m, exceptionParts(cls), &oneofs)
	for _, f := range cls.Fields {
		fp := &descriptorpb.FieldDescriptorProto{
			Name:     strPtr(f.Name),
			JsonName: strPtr(jsonName(f.Name)),
			Number:   int32Ptr(int32(f.Number)),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		switch f.Type.Kind {
		case "list", "set":
			fp.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			setType(fp, f.Type.Elem, cls)
		case "map":
			entry := &descriptorpb.DescriptorProto{
				Name:    strPtr(camel(f.Name) + "Entry"),
				Options: &descriptorpb.MessageOptions{MapEntry: boolPtr(true)},
			}
			key := &descriptorpb.FieldDescriptorProto{Name: strPtr("key"), JsonName: strPtr("key"), Number: int32Ptr(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()}
			setType(key, object.ScalarField(f.Type.Key), cls)
			value := &descriptorpb.FieldDescriptorProto{Name: strPtr("value"), JsonName: strPtr("value"), Number: int32Ptr(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()}
			setType(value, f.Type.Elem, cls)
			entry.Field = []*descriptorpb.FieldDescriptorProto{key, value}
			m.NestedType = append(m.NestedType, entry)
			fp.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			fp.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fp.TypeName = strPtr("." + Package(cls) + "." + cls.Name + "." + entry.GetName())
		case "enum":
			enum := &descriptorpb.EnumDescriptorProto{Name: strPtr(camel(f.Name))}
			prefix := strings.ToUpper(f.Name) + "_"
			enum.Value = append(enum.Value, &descriptorpb.EnumValueDescriptorProto{Name: strPtr(prefix + "UNSPECIFIED"), Number: int32Ptr(0)})
			for i, v := range f.Type.Enum {
				enum.Value = append(enum.Value, &descriptorpb.EnumValueDescriptorProto{Name: strPtr(prefix + strings.ToUpper(v)), Number: int32Ptr(int32(i + 1))})
			}
			m.EnumType = append(m.EnumType, enum)
			fp.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
			fp.TypeName = strPtr("." + Package(cls) + "." + cls.Name + "." + enum.GetName())
			optional(fp, m, &oneofs)
		default:
			setType(fp, f.Type, cls)
			optional(fp, m, &oneofs)
		}
		m.Field = append(m.Field, fp)
	}
	return m
}

// optional marks a singular field proto3-optional, with the synthetic
// oneof that carries presence.
func optional(fp *descriptorpb.FieldDescriptorProto, m *descriptorpb.DescriptorProto, oneofs *int) {
	fp.Proto3Optional = boolPtr(true)
	fp.OneofIndex = int32Ptr(int32(*oneofs))
	m.OneofDecl = append(m.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: strPtr("_" + fp.GetName())})
	*oneofs++
}

func setType(fp *descriptorpb.FieldDescriptorProto, t *object.FieldType, cls *object.Class) {
	if t.Scalar != nil {
		// descriptorpb names each scalar type TYPE_ and its .proto name.
		fp.Type = descriptorpb.FieldDescriptorProto_Type(descriptorpb.FieldDescriptorProto_Type_value["TYPE_"+strings.ToUpper(t.Scalar.Proto)]).Enum()
		return
	}
	switch t.Kind {
	case "ref":
		fp.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		fp.TypeName = strPtr("." + fullName(t.Ref))
	}
}

// chain resolves the files a class depends on, and what they depend on.
type chain struct{ files []protoreflect.FileDescriptor }

func (c *chain) add(fd protoreflect.FileDescriptor) {
	for _, f := range c.files {
		if f == fd {
			return
		}
	}
	c.files = append(c.files, fd)
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		c.add(imports.Get(i).FileDescriptor)
	}
}

func (c *chain) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	for _, f := range c.files {
		if f.Path() == path {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file %s not found", path)
}

func (c *chain) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	for _, f := range c.files {
		if d := find(f, name); d != nil {
			return d, nil
		}
	}
	return nil, fmt.Errorf("descriptor %s not found", name)
}

func find(f protoreflect.FileDescriptor, name protoreflect.FullName) protoreflect.Descriptor {
	msgs := f.Messages()
	for i := 0; i < msgs.Len(); i++ {
		if d := findIn(msgs.Get(i), name); d != nil {
			return d
		}
	}
	return nil
}

func findIn(m protoreflect.MessageDescriptor, name protoreflect.FullName) protoreflect.Descriptor {
	if m.FullName() == name {
		return m
	}
	enums := m.Enums()
	for i := 0; i < enums.Len(); i++ {
		if enums.Get(i).FullName() == name {
			return enums.Get(i)
		}
	}
	nested := m.Messages()
	for i := 0; i < nested.Len(); i++ {
		if d := findIn(nested.Get(i), name); d != nil {
			return d
		}
	}
	return nil
}

// camel turns snake_case into CamelCase.
func camel(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if r == '_' {
			up = true
			continue
		}
		if up {
			b.WriteString(strings.ToUpper(string(r)))
			up = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// jsonName is protoc's JSON name: lowerCamelCase.
func jsonName(s string) string {
	c := camel(s)
	if c == "" {
		return c
	}
	return strings.ToLower(c[:1]) + c[1:]
}

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }
