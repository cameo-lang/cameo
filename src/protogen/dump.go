package protogen

import (
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// A dump is a paused run, as `cameo` writes it on ^C and `cameo resume`
// reads it back: the script, where it paused, and what its top level held.
// Each value travels in its envelope wire form, so a dump needs nothing but
// this file to be read. The design is in doc/design/pause.md.
//
//	message Dump {
//	  string file = 1; string source = 2;
//	  int32 next = 3; int32 line = 4;
//	  repeated Binding variables = 5; repeated Binding constants = 6;
//	  repeated Promise promises = 7;
//	  repeated string loaded = 8; bytes rand = 9;
//	}
//	message Binding { string name = 1; bytes value = 2; string class = 3; string missing = 4; }
//	message Promise { int64 id = 1; string method = 2; bytes value = 3; Failure failure = 4; bool awaited = 5; }
//	message Failure { string class = 1; string message = 2; int32 line = 3; }

// Dump is a paused run.
type Dump struct {
	// File and Source are the script: its path and its text, which resume
	// runs again from Next.
	File, Source string
	// Next is the index of the top-level statement the run continues with,
	// Line its line.
	Next, Line int
	// Variables are the top-level variables, Constants the constants the
	// script assigned (its classes come from running their definitions
	// again).
	Variables, Constants []Binding
	// Promises are the promises the run held, settled.
	Promises []PromiseState
	// Loaded names the libraries require loaded.
	Loaded []string
	// Rand is the state of the random number generator.
	Rand []byte
}

// Binding is one variable or constant. Value is the envelope wire form of
// its value; a class is named in Class instead; a value the envelope cannot
// carry has neither, and Missing says what it was.
type Binding struct {
	Name    string
	Value   []byte
	Class   string
	Missing string
}

// PromiseState is a settled promise: its result in Value, or the exception
// it failed with in Failure, and in Value too when the envelope carries it. Awaited is set when the script awaited it
// before the pause.
type PromiseState struct {
	ID      int64
	Method  string
	Value   []byte
	Failure *Failure
	Awaited bool
}

// Failure is the exception a promise failed with.
type Failure struct {
	Class, Message string
	Line           int
}

var (
	dumpOnce sync.Once
	dumpFile protoreflect.FileDescriptor
	dumpErr  error
)

func dumpDescriptor() (protoreflect.FileDescriptor, error) {
	dumpOnce.Do(func() {
		opt := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
		rep := descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		field := func(name string, num int32, typ descriptorpb.FieldDescriptorProto_Type, typeName string, label *descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
			f := &descriptorpb.FieldDescriptorProto{Name: strPtr(name), JsonName: strPtr(jsonName(name)), Number: int32Ptr(num), Type: typ.Enum(), Label: label}
			if typeName != "" {
				f.TypeName = strPtr(".cameo." + typeName)
			}
			return f
		}
		str := descriptorpb.FieldDescriptorProto_TYPE_STRING
		bytes := descriptorpb.FieldDescriptorProto_TYPE_BYTES
		msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
		dump := &descriptorpb.DescriptorProto{Name: strPtr("Dump"), Field: []*descriptorpb.FieldDescriptorProto{
			field("file", 1, str, "", opt),
			field("source", 2, str, "", opt),
			field("next", 3, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", opt),
			field("line", 4, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", opt),
			field("variables", 5, msg, "Binding", rep),
			field("constants", 6, msg, "Binding", rep),
			field("promises", 7, msg, "Promise", rep),
			field("loaded", 8, str, "", rep),
			field("rand", 9, bytes, "", opt),
		}}
		binding := &descriptorpb.DescriptorProto{Name: strPtr("Binding"), Field: []*descriptorpb.FieldDescriptorProto{
			field("name", 1, str, "", opt),
			field("value", 2, bytes, "", opt),
			field("class", 3, str, "", opt),
			field("missing", 4, str, "", opt),
		}}
		promise := &descriptorpb.DescriptorProto{Name: strPtr("Promise"), Field: []*descriptorpb.FieldDescriptorProto{
			field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT64, "", opt),
			field("method", 2, str, "", opt),
			field("value", 3, bytes, "", opt),
			field("failure", 4, msg, "Failure", opt),
			field("awaited", 5, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", opt),
		}}
		failure := &descriptorpb.DescriptorProto{Name: strPtr("Failure"), Field: []*descriptorpb.FieldDescriptorProto{
			field("class", 1, str, "", opt),
			field("message", 2, str, "", opt),
			field("line", 3, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", opt),
		}}
		fdp := &descriptorpb.FileDescriptorProto{
			Name:        strPtr("cameo/dump.proto"),
			Package:     strPtr("cameo"),
			Syntax:      strPtr("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{dump, binding, promise, failure},
		}
		dumpFile, dumpErr = protodesc.NewFile(fdp, nil)
	})
	return dumpFile, dumpErr
}

// MarshalDump is the wire form of a dump.
func MarshalDump(d *Dump) ([]byte, error) {
	fd, err := dumpDescriptor()
	if err != nil {
		return nil, err
	}
	md := fd.Messages().ByName("Dump")
	m := dynamicpb.NewMessage(md)
	setString(m, "file", d.File)
	setString(m, "source", d.Source)
	m.Set(md.Fields().ByName("next"), protoreflect.ValueOfInt32(int32(d.Next)))
	m.Set(md.Fields().ByName("line"), protoreflect.ValueOfInt32(int32(d.Line)))
	bindingMd := fd.Messages().ByName("Binding")
	for _, name := range []string{"variables", "constants"} {
		bindings := d.Variables
		if name == "constants" {
			bindings = d.Constants
		}
		list := m.Mutable(md.Fields().ByName(protoreflect.Name(name))).List()
		for _, b := range bindings {
			bm := dynamicpb.NewMessage(bindingMd)
			setString(bm, "name", b.Name)
			setString(bm, "class", b.Class)
			setString(bm, "missing", b.Missing)
			if b.Value != nil {
				bm.Set(bindingMd.Fields().ByName("value"), protoreflect.ValueOfBytes(b.Value))
			}
			list.Append(protoreflect.ValueOfMessage(bm))
		}
	}
	promiseMd := fd.Messages().ByName("Promise")
	failureMd := fd.Messages().ByName("Failure")
	promises := m.Mutable(md.Fields().ByName("promises")).List()
	for _, p := range d.Promises {
		pm := dynamicpb.NewMessage(promiseMd)
		pm.Set(promiseMd.Fields().ByName("id"), protoreflect.ValueOfInt64(p.ID))
		setString(pm, "method", p.Method)
		if p.Value != nil {
			pm.Set(promiseMd.Fields().ByName("value"), protoreflect.ValueOfBytes(p.Value))
		}
		if p.Failure != nil {
			fm := dynamicpb.NewMessage(failureMd)
			setString(fm, "class", p.Failure.Class)
			setString(fm, "message", p.Failure.Message)
			fm.Set(failureMd.Fields().ByName("line"), protoreflect.ValueOfInt32(int32(p.Failure.Line)))
			pm.Set(promiseMd.Fields().ByName("failure"), protoreflect.ValueOfMessage(fm))
		}
		pm.Set(promiseMd.Fields().ByName("awaited"), protoreflect.ValueOfBool(p.Awaited))
		promises.Append(protoreflect.ValueOfMessage(pm))
	}
	loaded := m.Mutable(md.Fields().ByName("loaded")).List()
	for _, lib := range d.Loaded {
		loaded.Append(protoreflect.ValueOfString(lib))
	}
	if d.Rand != nil {
		m.Set(md.Fields().ByName("rand"), protoreflect.ValueOfBytes(d.Rand))
	}
	return proto.Marshal(m)
}

// UnmarshalDump reads a dump from its wire form.
func UnmarshalDump(data []byte) (*Dump, error) {
	fd, err := dumpDescriptor()
	if err != nil {
		return nil, err
	}
	md := fd.Messages().ByName("Dump")
	m := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, m); err != nil {
		return nil, err
	}
	d := &Dump{
		File:   getString(m, "file"),
		Source: getString(m, "source"),
		Next:   int(m.Get(md.Fields().ByName("next")).Int()),
		Line:   int(m.Get(md.Fields().ByName("line")).Int()),
	}
	bindings := func(name string) []Binding {
		list := m.Get(md.Fields().ByName(protoreflect.Name(name))).List()
		var out []Binding
		for i := 0; i < list.Len(); i++ {
			bm := list.Get(i).Message()
			b := Binding{Name: getString(bm, "name"), Class: getString(bm, "class"), Missing: getString(bm, "missing")}
			if bm.Has(bm.Descriptor().Fields().ByName("value")) {
				b.Value = bm.Get(bm.Descriptor().Fields().ByName("value")).Bytes()
			}
			out = append(out, b)
		}
		return out
	}
	d.Variables = bindings("variables")
	d.Constants = bindings("constants")
	promises := m.Get(md.Fields().ByName("promises")).List()
	for i := 0; i < promises.Len(); i++ {
		pm := promises.Get(i).Message()
		pmd := pm.Descriptor()
		p := PromiseState{
			ID:      pm.Get(pmd.Fields().ByName("id")).Int(),
			Method:  getString(pm, "method"),
			Awaited: pm.Get(pmd.Fields().ByName("awaited")).Bool(),
		}
		if pm.Has(pmd.Fields().ByName("value")) {
			p.Value = pm.Get(pmd.Fields().ByName("value")).Bytes()
		}
		if pm.Has(pmd.Fields().ByName("failure")) {
			fm := pm.Get(pmd.Fields().ByName("failure")).Message()
			p.Failure = &Failure{
				Class:   getString(fm, "class"),
				Message: getString(fm, "message"),
				Line:    int(fm.Get(fm.Descriptor().Fields().ByName("line")).Int()),
			}
		}
		d.Promises = append(d.Promises, p)
	}
	loaded := m.Get(md.Fields().ByName("loaded")).List()
	for i := 0; i < loaded.Len(); i++ {
		d.Loaded = append(d.Loaded, loaded.Get(i).String())
	}
	if m.Has(md.Fields().ByName("rand")) {
		d.Rand = m.Get(md.Fields().ByName("rand")).Bytes()
	}
	return d, nil
}

// setString sets a string field of m, leaving it unset for "".
func setString(m *dynamicpb.Message, name string, s string) {
	if s != "" {
		m.Set(m.Descriptor().Fields().ByName(protoreflect.Name(name)), protoreflect.ValueOfString(s))
	}
}

func getString(m protoreflect.Message, name string) string {
	return m.Get(m.Descriptor().Fields().ByName(protoreflect.Name(name))).String()
}
