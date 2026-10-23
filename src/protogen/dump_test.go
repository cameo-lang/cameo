package protogen

import (
	"reflect"
	"testing"
)

// TestDumpRoundTrips proves a dump reads back as it was written, with a
// missing binding, a class binding, a failed promise and the generator's
// state all kept.
func TestDumpRoundTrips(t *testing.T) {
	d := &Dump{
		File: "x.cameo", Source: "puts 1\n", Next: 1, Line: 2,
		Variables: []Binding{{Name: "a", Value: []byte{0x10, 0x01}}, {Name: "f", Missing: "a Proc cannot be serialised"}},
		Constants: []Binding{{Name: "P", Class: "Geo::Point"}},
		Promises: []PromiseState{
			{ID: 1, Method: "x", Value: []byte{0x10, 0x02}, Awaited: true},
			{ID: 2, Method: "boom", Failure: &Failure{Class: "ArgumentError", Message: "boom", Line: 3}},
		},
		Loaded: []string{"json", "set"},
		Rand:   []byte("pcg:0123456789abcdef"),
	}
	data, err := MarshalDump(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalDump(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, d) {
		t.Fatalf("read back %+v, wrote %+v", got, d)
	}
}
