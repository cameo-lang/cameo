package protogen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/radiospiel/amber/src/object"
)

// person builds a class the way the evaluator would: Person with a ref to
// Address, inside module Geo.
func person() (*object.Class, *object.Class) {
	geo := object.NewModule("Geo")
	geo.Digest = "m"
	address := object.NewClass("Address", object.ObjectClass)
	address.Digest = "a"
	address.Lexical = geo
	addField := func(cls *object.Class, name string, number int64, t *object.FieldType) {
		f := &object.Field{Name: name, Number: number, Type: t, Owner: cls}
		cls.FieldIndex[name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
	addField(address, "city", 1, &object.FieldType{Kind: "string"})
	p := object.NewClass("Person", object.ObjectClass)
	p.Digest = "p"
	p.Lexical = geo
	addField(p, "name", 1, &object.FieldType{Kind: "string"})
	p.ReservedNumbers = []int64{2}
	p.ReservedNames = []string{"nick"}
	addField(p, "home", 3, &object.FieldType{Kind: "ref", Ref: address})
	addField(p, "emails", 4, &object.FieldType{Kind: "list", Elem: &object.FieldType{Kind: "string"}})
	addField(p, "scores", 5, &object.FieldType{Kind: "map", Key: "string", Elem: &object.FieldType{Kind: "int"}})
	addField(p, "role", 6, &object.FieldType{Kind: "enum", Enum: []string{"admin", "user"}})
	return p, address
}

func TestDescriptorAndText(t *testing.T) {
	p, _ := person()
	reg := NewRegistry()
	md, err := reg.Message(p)
	if err != nil {
		t.Fatal(err)
	}
	if md.FullName() != "geo.Person" {
		t.Fatalf("full name %s", md.FullName())
	}
	if md.Fields().ByName("home").Message().FullName() != "geo.Address" {
		t.Fatalf("home refers to %s", md.Fields().ByName("home").Message().FullName())
	}
	if !md.Fields().ByName("scores").IsMap() {
		t.Fatal("scores is a map")
	}
	if !md.Fields().ByName("name").HasPresence() {
		t.Fatal("a scalar field has presence")
	}
	want := `package geo;

message Person {
  enum Role {
    ROLE_UNSPECIFIED = 0;
    ROLE_ADMIN = 1;
    ROLE_USER = 2;
  }
  reserved 2;
  reserved "nick";
  optional string name = 1;
  optional Address home = 3;
  repeated string emails = 4;
  map<string, int64> scores = 5;
  optional Role role = 6;
}
`
	if got := Text(p); got != want {
		t.Fatalf("text:\n%s\nwant:\n%s", got, want)
	}
	files := Files([]*object.Class{p})
	if !strings.Contains(files["geo.proto"], "syntax = \"proto3\";\n\npackage geo;\n") {
		t.Fatalf("file:\n%s", files["geo.proto"])
	}
}

func TestInstanceRoundTrip(t *testing.T) {
	p, address := person()
	reg := NewRegistry()
	home := &object.Instance{Cls: address, Fields: []object.Value{object.String("Springfield")}}
	ann := &object.Instance{Cls: p, Fields: []object.Value{
		object.String("ann"), object.InstanceValue(home),
		object.NewList(object.String("a"), object.String("b")),
		object.MapValue(object.MapOf(object.String("math"), object.Int(1))),
		object.Symbol("user"),
	}}
	data, err := reg.Marshal(ann)
	if err != nil {
		t.Fatal(err)
	}
	back, err := reg.Unmarshal(p, data)
	if err != nil {
		t.Fatal(err)
	}
	if object.Key(object.InstanceValue(back)) != object.Key(object.InstanceValue(ann)) {
		t.Fatalf("round trip changed the value: %s vs %s", object.Key(object.InstanceValue(back)), object.Key(object.InstanceValue(ann)))
	}
	msg, err := reg.ToMessage(ann)
	if err != nil {
		t.Fatal(err)
	}
	if ann.Message != msg {
		t.Fatal("the message is kept on the instance")
	}
	if got := msg.Get(msg.Descriptor().Fields().ByName("name")).String(); got != "ann" {
		t.Fatalf("name field reads %q", got)
	}
	var _ proto.Message = msg
	var _ protoreflect.Message = msg.ProtoReflect()
}

func TestEnvelopeRoundTrip(t *testing.T) {
	p, _ := person()
	reg := NewRegistry()
	resolve := func(name, digest string) *object.Class {
		if name == "Geo::Person" && digest == "p" {
			return p
		}
		return nil
	}
	enc, dec := NewEncoder(reg), NewDecoder(reg, resolve)
	values := []object.Value{
		object.Nil, object.True, object.Int(-7), object.Float(2.5), object.String("s"), object.Bytes([]byte("b")),
		object.NewList(object.Int(1), object.NewList()), object.MapValue(object.MapOf(object.String("k"), object.Int(1))),
		object.SetValue(object.NewSet(object.Int(1), object.Int(2))), object.RangeValue(&object.Range{Lo: object.Int(1), Hi: object.Int(3)}),
		object.InstanceValue(&object.Instance{Cls: p, Fields: []object.Value{object.String("ann"), {}, object.NewList(), object.MapValue(object.NewMap()), {}}}),
	}
	for _, v := range values {
		data, err := enc.Encode(v)
		if err != nil {
			t.Fatalf("%s: %v", object.Key(v), err)
		}
		back, err := dec.Decode(data)
		if err != nil {
			t.Fatalf("%s: %v", object.Key(v), err)
		}
		if object.Key(back) != object.Key(v) {
			t.Fatalf("round trip changed %s into %s", object.Key(v), object.Key(back))
		}
	}
	data, err := enc.Encode(object.Symbol("sym"))
	if err != nil {
		t.Fatal(err)
	}
	if back, _ := dec.Decode(data); !object.Equal(back, object.String("sym")) {
		t.Fatalf("a symbol travels as a string, got %s", object.Key(back))
	}
	if _, err := enc.Encode(object.ClassValue(object.ProcClass)); err == nil {
		t.Fatal("a class cannot be encoded")
	}
}
