package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Time.strptime reads %s from a string with whitespace after it, such as
// a line of a file with its newline, as CRuby does.
func TestTimeStrptimeReadsEpochSecondsBeforeWhitespace(t *testing.T) {
	out := runScript(t, `require "time"
p Time.strptime("1700000000\n", "%s").to_i
p Time.strptime("1700000000 \t", "%s").to_i
p Time.strptime("1700000000.5\n", "%s.%N").to_i
`)
	assert.Equals(t, out, "1700000000\n1700000000\n1700000000\n")
}
