package protogen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/cameo-lang/cameo/src/object"
)

// geoClass builds a class in module Geo, its fields numbered as given.
func geoClass(name string, fields ...*object.Field) *object.Class {
	geo := object.NewModule("Geo")
	geo.Digest = "m"
	cls := object.NewClass(name, object.ObjectClass)
	cls.Digest = name
	cls.Lexical = geo
	for _, f := range fields {
		f.Owner = cls
		cls.FieldIndex[f.Name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
	return cls
}

func message(t *testing.T, cls *object.Class) protoreflect.MessageDescriptor {
	t.Helper()
	md, err := NewRegistry().Message(cls)
	if err != nil {
		t.Fatal(err)
	}
	return md
}

func TestMergeAddsTheFieldsTheRegisteredDefinitionLacks(t *testing.T) {
	address := geoClass("Address", &object.Field{Name: "city", Number: 1, Type: object.ScalarField("string")})
	registered := geoClass("Person",
		&object.Field{Name: "name", Number: 1, Type: object.ScalarField("string")},
		&object.Field{Name: "home", Number: 3, Type: &object.FieldType{Kind: "ref", Ref: address}},
		&object.Field{Name: "emails", Number: 4, Type: &object.FieldType{Kind: "list", Elem: object.ScalarField("string")}},
	)
	registered.ReservedNumbers = []int64{2}
	// An older definition: no emails, a role and scores of its own, the
	// role at a number the registered one reserves, and home's Address
	// without its city.
	older := geoClass("Person",
		&object.Field{Name: "name", Number: 1, Type: object.ScalarField("string")},
		&object.Field{Name: "role", Number: 2, Type: &object.FieldType{Kind: "enum", Enum: []string{"admin", "user"}}},
		&object.Field{Name: "home", Number: 3, Type: &object.FieldType{Kind: "ref", Ref: geoClass("Address")}},
		&object.Field{Name: "scores", Number: 5, Type: &object.FieldType{Kind: "map", Key: "string", Elem: object.ScalarField("int")}},
	)
	file, err := Merge(message(t, registered), message(t, older))
	if err != nil {
		t.Fatal(err)
	}
	fd, err := protodesc.NewFile(file, resolver(message(t, registered).ParentFile()))
	if err != nil {
		t.Fatal(err)
	}
	merged := fd.Messages().ByName("Person")
	none := func(int64) bool { return false }
	for name, want := range map[protoreflect.Name]int64{
		"name": 1, "home": 3, "emails": 4,
		"role": FieldNumber("role", none), "scores": FieldNumber("scores", none),
	} {
		f := merged.Fields().ByName(name)
		if f == nil || int64(f.Number()) != want {
			t.Fatalf("%s: %v, want number %d", name, f, want)
		}
	}
	if !merged.Fields().ByName("role").HasPresence() || merged.Enums().ByName("Role") == nil || !merged.Fields().ByName("scores").IsMap() {
		t.Fatalf("role and scores keep their types: %v", merged)
	}

	doc := dynamicpb.NewMessage(message(t, older))
	if err := protojson.Unmarshal([]byte(`{"name":"a","role":"ROLE_ADMIN","scores":{"x":"1"},"home":{}}`), doc); err != nil {
		t.Fatal(err)
	}
	json, err := protojson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(json, dynamicpb.NewMessage(merged)); err != nil {
		t.Fatalf("the merged message reads the older one's JSON: %v", err)
	}

	again, err := Merge(merged, message(t, older))
	if err != nil {
		t.Fatal(err)
	}
	if len(again.MessageType[0].Field) != 5 {
		t.Fatal("a definition that only lacks fields adds none")
	}
}

func TestMergeRefusesAChangedField(t *testing.T) {
	registered := message(t, geoClass("Person",
		&object.Field{Name: "name", Number: 1, Type: object.ScalarField("string")},
		&object.Field{Name: "home", Number: 2, Type: &object.FieldType{Kind: "ref", Ref: geoClass("Address", &object.Field{Name: "city", Number: 1, Type: object.ScalarField("string")})}},
		&object.Field{Name: "role", Number: 3, Type: &object.FieldType{Kind: "enum", Enum: []string{"admin"}}},
	))
	for want, next := range map[string]*object.Class{
		"Person.name: string cannot become int64":    geoClass("Person", &object.Field{Name: "name", Number: 1, Type: object.ScalarField("int")}),
		"Person.name: a field keeps its cardinality": geoClass("Person", &object.Field{Name: "name", Number: 1, Type: &object.FieldType{Kind: "list", Elem: object.ScalarField("string")}}),
		"Address.zip = 2 is not in the registered definition": geoClass("Person", &object.Field{Name: "home", Number: 2, Type: &object.FieldType{Kind: "ref", Ref: geoClass("Address",
			&object.Field{Name: "city", Number: 1, Type: object.ScalarField("string")},
			&object.Field{Name: "zip", Number: 2, Type: object.ScalarField("int")},
		)}}),
		"the enum geo.Person.Role differs": geoClass("Person", &object.Field{Name: "role", Number: 3, Type: &object.FieldType{Kind: "enum", Enum: []string{"admin", "user"}}}),
	} {
		_, err := Merge(registered, message(t, next))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("got %v, want %q", err, want)
		}
	}
}
