package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// A Go host gives text as a string, which must be UTF-8, and bytes as a
// []byte, which the script gets as a binary String.
func TestAHostGivesTextAsAStringAndBytesAsBytes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prog, err := cameo.LoadFromString("t.cameo", `def kind(s) = s.encoding.to_s`, cameoio.Streams(&stdout, &stderr))
	assert.NoError(t, err)
	assert.NoError(t, prog.Run())
	got, err := prog.Call(nil, "kind", "héllo")
	assert.NoError(t, err)
	assert.Equals(t, got, any("UTF-8"))
	got, err = prog.Call(nil, "kind", []byte{0xff})
	assert.NoError(t, err)
	assert.Equals(t, got, any("ASCII-8BIT"))
	_, err = prog.Call(nil, "kind", "\xff")
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), `a Go string that is not UTF-8: "\xff"; give a []byte for bytes`)
}
