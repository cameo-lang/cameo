package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// A String made as text of bytes that are not UTF-8 is refused where it
// is made, and the script gets an ArgumentError it can rescue: here a
// StringIO of text whose buffer was given a byte that is none.
func TestBytesThatAreNotTheEncodingRaiseAnArgumentError(t *testing.T) {
	out := runScript(t, `require "stringio"
io = StringIO.new("abc")
io.ungetbyte(255)
begin
  io.read
rescue ArgumentError => e
  puts e.message
end
puts "went on"
`)
	assert.Equals(t, out, "invalid byte sequence in UTF-8: \"\\xffabc\" is not text; keep such bytes in a binary string\nwent on\n")
}

// A Go host gives text as a string, which must be UTF-8, and bytes as a
// []byte, which the script gets as a binary String.
func TestAHostGivesTextAsAStringAndBytesAsBytes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", `def kind(s) = s.encoding.to_s`, &stdout, &stderr)
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
