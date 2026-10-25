package protogen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/object"
)

// httpError builds an exception class the way the evaluator would:
// HTTPError < StandardError with an int field.
func httpError() *object.Class {
	cls := object.NewClass("HTTPError", object.StandardError)
	cls.Digest = "h"
	f := &object.Field{Name: "status", Number: 100, Type: object.ScalarField("int"), Owner: cls}
	cls.FieldIndex["status"] = 0
	cls.Fields = []*object.Field{f}
	return cls
}

func TestAnExceptionClassIsAMessageOfItsPartsAndFields(t *testing.T) {
	cls := httpError()
	md, err := NewRegistry().Message(cls)
	if err != nil {
		t.Fatal(err)
	}
	if md.FullName() != "cameo.HTTPError" {
		t.Fatalf("full name %s", md.FullName())
	}
	if got := md.Fields().ByName("cause").Message().FullName(); got != "cameo.Value" {
		t.Fatalf("cause is a %s", got)
	}
	want := `message HTTPError {
  optional string message = 1;
  optional int64 line = 2;
  optional cameo.Value cause = 3;
  optional int64 status = 100;
}
`
	if got := Text(cls); got != want {
		t.Fatalf("text:\n%s\nwant:\n%s", got, want)
	}
	file := Files([]*object.Class{cls})["cameo.proto"]
	if !strings.Contains(file, "import \"cameo/value.proto\";\n") {
		t.Fatalf("file:\n%s", file)
	}
}

func TestABuiltinExceptionIsAMessageOfWhatItCarries(t *testing.T) {
	md, err := NewRegistry().Message(object.NoMethodError)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for i := 0; i < md.Fields().Len(); i++ {
		names = append(names, string(md.Fields().Get(i).Name()))
	}
	if got := strings.Join(names, " "); got != "message line cause name receiver args" {
		t.Fatalf("NoMethodError's fields: %s", got)
	}
	if md.FullName() != "cameo.NoMethodError" {
		t.Fatalf("full name %s", md.FullName())
	}
}

func TestAnExceptionRoundTripsThroughTheEnvelope(t *testing.T) {
	cls := httpError()
	reg := NewRegistry()
	resolve := func(name, digest string) *object.Class {
		switch name {
		case "HTTPError":
			return cls
		case "RuntimeError":
			return object.RuntimeError
		}
		return nil
	}
	cause := &object.Exception{Cls: object.RuntimeError, Message: "inner", Line: 3}
	exc := &object.Exception{Cls: cls, Message: "HTTP 404", Line: 7, Cause: object.ExceptionValue(cause),
		Fields: []object.Value{object.Int(404)}}
	data, err := NewEncoder(reg).Encode(object.ExceptionValue(exc))
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewDecoder(reg, resolve).Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	back, ok := v.AsException()
	if !ok {
		t.Fatalf("decoded a %s", v.Kind())
	}
	if !object.Equal(v, object.ExceptionValue(exc)) || back.Line != 7 {
		t.Fatalf("round trip changed the exception: %s line %d", object.Key(v), back.Line)
	}
	c, ok := back.Cause.AsException()
	if !ok || c.Message != "inner" || c.Line != 3 {
		t.Fatalf("cause %v", back.Cause)
	}
}

func TestAPartTheEnvelopeCannotCarryIsLeftOut(t *testing.T) {
	reg := NewRegistry()
	exc := &object.Exception{Cls: object.NameError, Message: "undefined", Attrs: map[string]object.Value{
		"name": object.Symbol("x"), "receiver": object.ClassValue(object.IntegerClass),
	}}
	msg, err := reg.ExceptionMessage(exc)
	if err != nil {
		t.Fatal(err)
	}
	fields := msg.Descriptor().Fields()
	if msg.Has(fields.ByName("receiver")) {
		t.Fatal("a class receiver was stored")
	}
	if got := msg.Get(fields.ByName("name")).String(); got != "x" {
		t.Fatalf("name is %q", got)
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	back, err := NewDecoder(reg, nil).UnmarshalException(object.NameError, data)
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := back.Attr("name"); name.Kind() != object.KindSymbol {
		t.Fatalf("name reads back as a %s", name.Kind())
	}
	if _, ok := back.Attr("receiver"); ok {
		t.Fatal("a receiver came back")
	}
}
