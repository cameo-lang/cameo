package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// b"..." is a binary string whatever its bytes are, with the escapes of
// a double-quoted string, and b'...' with those of a single-quoted one.
func TestABinaryStringLiteralIsItsBytes(t *testing.T) {
	out := runScript(t, `
x = b"a\xFFb"
p x, x.encoding, x.bytes, b'q\xFF', b"caf\xC3\xA9".encoding, b"".encoding
p b"abc" == "abc", b"\xC3" + b"\xA9", { b"k" => 1 }[b"k"], [b"\x80", b"\x81"].map(&:bytes)
def tag(s, n) = "#{n}: #{s.bytesize}"
t = tag b"\x00", 1
puts tag(b"\x00\x01", 2), t
`)
	assert.Equals(t, out, `"a\xFFb"
#<Encoding:BINARY (ASCII-8BIT)>
[97, 255, 98]
"q\\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
true
"\xC3\xA9"
1
[[128], [129]]
2: 2
1: 1
`)
}

// A string literal with bytes that are not UTF-8 is refused before the
// script runs, and the message names the literal to write.
func TestAStringLiteralOfOtherBytesIsRefused(t *testing.T) {
	for src, line := range map[string]string{`p "\xFF"`: "1", `p "a#{1}\xC3"`: "1", "puts 1\np \"\\xFF\"": "2"} {
		var stdout, stderr bytes.Buffer
		err := cameo.Run("t.cameo", src+"\n", &stdout, &stderr)
		assert.Error(t, err, "t.cameo:"+line+`: a string literal with bytes that are not UTF-8 is a binary string and is written b"..."`)
		assert.Equals(t, stdout.String(), "")
	}
}
