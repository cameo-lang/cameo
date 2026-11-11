package cameo

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// An instance with a proc field comes to Go as its message, the proc a
// cameo.Closure in it, and goes back as an instance whose proc runs: a
// host can keep such a value, or its bytes, between two calls.
func TestAnInstanceWithAProcFieldGoesToGoAndBack(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", `class Route
  string :path
  proc :handler
end
def route(n) = Route.new(path: "/a", handler: proc { |x| x * n })
def serve(route, x) = route.handler.call(x)
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	route, err := prog.Call(nil, "route", 7)
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := route.(proto.Message)
	if !ok {
		t.Fatalf("the instance came back as a %T, want a proto.Message", route)
	}
	md := msg.ProtoReflect().Descriptor()
	fd := md.Fields().ByName("handler")
	if fd == nil || fd.Kind() != protoreflect.MessageKind || fd.Message().FullName() != "cameo.Closure" {
		t.Fatalf("the handler field of %s is %v, want a cameo.Closure", md.FullName(), fd)
	}
	got, err := prog.Call(nil, "serve", msg, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("serve returned %#v, want 42", got)
	}
	// As bytes, which a host stores, and reads into the class's message.
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	stored := msg.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(data, stored); err != nil {
		t.Fatal(err)
	}
	got, err = prog.Call(nil, "serve", stored, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(14) {
		t.Fatalf("serve returned %#v for the stored route, want 14", got)
	}
}
