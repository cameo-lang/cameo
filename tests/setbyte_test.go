package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// setbyte sets a byte of a binary string and refuses a string of
// characters, UTF-8 or US-ASCII, where Ruby sets the byte in any. The
// script runs with mutation errors ignored, so that the call reaches the
// method.
func TestSetbyteTakesABinaryStringAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", `p b"ab".setbyte(0, 65)
["ab", "é", "ab".force_encoding("US-ASCII")].each do |s|
  begin
    s.setbyte(0, 65)
  rescue Encoding::CompatibilityError => e
    puts e.message
  end
end
`, &stdout, &stderr)
	assert.NoError(t, err)
	prog.IgnoreMutationErrors(true)
	assert.NoError(t, prog.Run())
	refusal := "setbyte sets a byte, and only a binary string has bytes alone: call b first\n"
	assert.Equals(t, stdout.String(), "65\n"+refusal+refusal+refusal)
}
