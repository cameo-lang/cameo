package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// Each test here is a path that handles a binary String, or bytes that
// are not UTF-8, and raised when Strings began to be checked where they
// are made: a review found them.

// A String made as text of bytes that are none, where no method has an
// error of its own, raises an ArgumentError with the line it happened
// on and a message that says what the bytes are: here a StringIO of text
// read from the middle of a character.
func TestBytesMadeIntoTextRaiseWithALine(t *testing.T) {
	const script = `require "stringio"
io = StringIO.new("héllo")
io.read(2)
io.gets
`
	const message = `invalid byte sequence in UTF-8: "\xa9llo" is not text; keep such bytes in a binary string (see https://15b.it/inv)`
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", script, cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:4: "+message+" (ArgumentError)")
}
