package cameopb

import (
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/assert"
)

// earlierDump is the wire form of the dump below as cameo wrote it while
// the dump's descriptor was built by hand, before dump.proto.
const earlierDump = "12077075747320310a1801320f1a0a47656f3a3a506f696e740a01503a0b08011201781a02100228013a211204626f6f6d221718030a0d417267756d656e744572726f721204626f6f6d08020a07782e63616d656f20022a070a0161120210012a20221b612050726f632063616e6e6f742062652073657269616c697365640a016642046a736f6e42037365744a147063673a30313233343536373839616263646566"

// A dump written before dump.proto reads as the same dump: the field
// numbers are the ones it had, with a missing binding, a class binding, a
// failed promise and the generator's state all kept.
func TestDumpReadsItsEarlierWireForm(t *testing.T) {
	want := &Dump{
		File: "x.cameo", Source: "puts 1\n", Next: 1, Line: 2,
		Variables: []*Dump_Binding{{Name: "a", Value: []byte{0x10, 0x01}}, {Name: "f", Missing: "a Proc cannot be serialised"}},
		Constants: []*Dump_Binding{{Name: "P", Class: "Geo::Point"}},
		Promises: []*Dump_Promise{
			{Id: 1, Method: "x", Value: []byte{0x10, 0x02}, Awaited: true},
			{Id: 2, Method: "boom", Failure: &Dump_Failure{Class: "ArgumentError", Message: "boom", Line: 3}},
		},
		Loaded: []string{"json", "set"},
		Rand:   []byte("pcg:0123456789abcdef"),
	}
	raw, err := hex.DecodeString(earlierDump)
	assert.NoError(t, err)
	got := &Dump{}
	assert.NoError(t, proto.Unmarshal(raw, got))
	assert.True(t, proto.Equal(got, want), "read %v, want %v", got, want)

	data, err := proto.Marshal(want)
	assert.NoError(t, err)
	again := &Dump{}
	assert.NoError(t, proto.Unmarshal(data, again))
	assert.True(t, proto.Equal(again, want), "read back %v, wrote %v", again, want)
}
