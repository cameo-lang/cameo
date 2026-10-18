package cameo

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The tests' messages: example.geo.Point and Path, built here since the
// tests have no protoc, and timestamppb.Timestamp, a generated type.
var pointMD, pathMD protoreflect.MessageDescriptor

func init() {
	pointMD, pathMD = geoMessages()
	field := func(m *dynamicpb.Message, name string) protoreflect.FieldDescriptor {
		return m.Descriptor().Fields().ByName(protoreflect.Name(name))
	}
	Register("Shapes::Point", dynamicpb.NewMessage(pointMD)).
		DefineMethod("norm2", func(self *dynamicpb.Message, args []any, kwargs map[string]any) (any, error) {
			x, y := self.Get(field(self, "x")).Int(), self.Get(field(self, "y")).Int()
			return x*x + y*y, nil
		}).
		DefineMethod("moved", func(self *dynamicpb.Message, args []any, kwargs map[string]any) (any, error) {
			// Changing self changes the copy; the copy comes back as a new
			// instance.
			self.Set(field(self, "x"), protoreflect.ValueOfInt64(self.Get(field(self, "x")).Int()+args[0].(int64)))
			return self, nil
		}).
		DefineClassMethod("origin", func(args []any, kwargs map[string]any) (any, error) {
			return dynamicpb.NewMessage(pointMD), nil
		})
	Register("Shapes::Path", dynamicpb.NewMessage(pathMD))
	Register("Google::Protobuf::Timestamp", &timestamppb.Timestamp{}).
		DefineMethod("to_f", func(self *timestamppb.Timestamp, args []any, kwargs map[string]any) (any, error) {
			return float64(self.Seconds) + float64(self.Nanos)/1e9, nil
		})
}

func geoMessages() (protoreflect.MessageDescriptor, protoreflect.MessageDescriptor) {
	str := proto.String
	num := proto.Int32
	label := func(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label { return &l }
	typ := func(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type { return &t }
	opt, rep := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	fdp := &descriptorpb.FileDescriptorProto{
		Name: str("example/geo.proto"), Package: str("example.geo"), Syntax: str("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: str("Color"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: str("COLOR_UNSPECIFIED"), Number: num(0)},
				{Name: str("COLOR_RED"), Number: num(1)},
				{Name: str("COLOR_GREEN"), Number: num(2)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: str("Point"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: str("x"), Number: num(1), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_INT64), JsonName: str("x")},
				{Name: str("y"), Number: num(2), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_INT64), JsonName: str("y")},
				{Name: str("label"), Number: num(3), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: str("label")},
			}},
			{Name: str("Path"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: str("points"), Number: num(1), Label: label(rep), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: str(".example.geo.Point"), JsonName: str("points")},
					{Name: str("tags"), Number: num(2), Label: label(rep), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: str(".example.geo.Path.TagsEntry"), JsonName: str("tags")},
					{Name: str("color"), Number: num(3), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_ENUM), TypeName: str(".example.geo.Color"), JsonName: str("color")},
					{Name: str("width"), Number: num(4), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_INT32), JsonName: str("width"), Proto3Optional: proto.Bool(true), OneofIndex: num(0)},
					{Name: str("next"), Number: num(5), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: str(".example.geo.Path"), JsonName: str("next")},
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: str("_width")}},
				NestedType: []*descriptorpb.DescriptorProto{{
					Name:    str("TagsEntry"),
					Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
					Field: []*descriptorpb.FieldDescriptorProto{
						{Name: str("key"), Number: num(1), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: str("key")},
						{Name: str("value"), Number: num(2), Label: label(opt), Type: typ(descriptorpb.FieldDescriptorProto_TYPE_INT64), JsonName: str("value")},
					},
				}},
			},
		},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		panic(err)
	}
	return fd.Messages().ByName("Point"), fd.Messages().ByName("Path")
}

// native runs src and returns what it printed.
func native(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil {
		t.Fatalf("%v\noutput: %s", err, out.String())
	}
	return out.String()
}

func TestNativeClassFieldsAndMethods(t *testing.T) {
	out := native(t, `pt = Shapes::Point.new(x: 3, y: 4)
p pt.x, pt.label, pt.norm2
q = pt.moved(1)
p q.x, pt.x
p Shapes::Point.origin.x
p pt.with(x: 9).x
p pt == Shapes::Point.new(x: 3, y: 4)
p pt.class, Shapes.class
`)
	want := "3\n\"\"\n25\n4\n3\n0\n9\ntrue\nShapes::Point\nModule\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

func TestNativeCollectionsEnumsAndRefs(t *testing.T) {
	out := native(t, `path = Shapes::Path.new(points: [Shapes::Point.new(x: 1), Shapes::Point.origin], tags: {"a" => 1}, color: :red)
p path.points.size, path.points.first.x, path.tags, path.color, path.width, path.next
longer = path.with(next: Shapes::Path.new(color: :green))
p longer.next.color, path.next
`)
	want := "2\n1\n{\"a\" => 1}\n:red\nnil\nnil\n:green\nnil\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

func TestNativeInstancesCrossToGo(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", `
def stamp = Google::Protobuf::Timestamp.new(seconds: 5, nanos: 500_000_000)
def secs(t) = t.to_f
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prog.Call(nil, "stamp")
	if err != nil {
		t.Fatal(err)
	}
	ts, ok := got.(*timestamppb.Timestamp)
	if !ok || ts.Seconds != 5 || ts.Nanos != 500_000_000 {
		t.Fatalf("stamp: %#v", got)
	}
	secs, err := prog.Call(nil, "secs", &timestamppb.Timestamp{Seconds: 2, Nanos: 250_000_000})
	if err != nil || secs != 2.25 {
		t.Fatalf("secs: %v, %v", secs, err)
	}
	origin, err := prog.Call("Shapes::Point", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := origin.(*dynamicpb.Message); !ok || m.Descriptor().FullName() != "example.geo.Point" {
		t.Fatalf("origin: %#v", origin)
	}
}

func TestNativeInstancesCrossToWorkers(t *testing.T) {
	out := native(t, `class Job
  int :factor
  def run(pt) = pt.norm2 * factor
end
job = Job.new(factor: 2)
p await(async(job, :run, Shapes::Point.new(x: 1, y: 2)))
`)
	if out != "10\n" {
		t.Errorf("output %q", out)
	}
}

func TestNativeNamespacesAreClosed(t *testing.T) {
	for _, src := range []string{"module Shapes\nend\n", "class Shapes::Point\nend\n", "class Shapes::Point::Inner\nend\n", "module Math::Extra\nend\n"} {
		var out bytes.Buffer
		prog, err := Load("t.cameo", src, &out, &out)
		if err != nil {
			t.Fatal(err)
		}
		if err := prog.Run(); err == nil || !strings.Contains(err.Error(), "cannot") {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestProgramModuleKeepsRegisteredClasses(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", "p Shapes.hello, Shapes::Point.origin.x\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Module("Shapes").Def("hello", func(args []any, kwargs map[string]any) (any, error) { return "hi", nil })
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "\"hi\"\n0\n" {
		t.Errorf("output %q", out.String())
	}
}

func TestRegisterRefuses(t *testing.T) {
	refuses := func(name string, want string, register func()) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil || !strings.Contains(r.(string), want) {
				t.Errorf("%s: panic %v, want %q", name, r, want)
			}
		}()
		register()
	}
	// The tests above loaded programs, so a registration now is too late.
	refuses("after Load", "before the first Load", func() { Register("Late::Point", dynamicpb.NewMessage(pointMD)) })
}

func TestRegisterChecksNamesAndMessages(t *testing.T) {
	// What Register refuses before it registers anything, tried with the
	// registry still open.
	was := loaded.Swap(false)
	defer loaded.Store(was)
	for _, c := range []struct {
		name string
		msg  proto.Message
		want string
	}{
		{"geo::point", &durationpb.Duration{}, "is not a constant name"},
		{"Shapes::Again", dynamicpb.NewMessage(pointMD), "registered already"},
		{"Math::Span", &durationpb.Duration{}, "builtin constant"},
		{"Shapes::Point", &durationpb.Duration{}, "defined already"},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(r.(string), c.want) {
					t.Errorf("%s: panic %v, want %q", c.name, r, c.want)
				}
			}()
			Register(c.name, c.msg)
		}()
	}
}
