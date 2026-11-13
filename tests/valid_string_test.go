package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A UTF-8 string is made of whole characters alone, unlike Ruby's. Bytes
// that are not UTF-8 are a binary string where they come in, as a
// literal or from pack, and calling them UTF-8 raises.
func TestAStringOfUTF8HoldsWholeCharactersAlone(t *testing.T) {
	out := runScript(t, `
p "\xC3".encoding, "caf\xC3\xA9".encoding, "caf\xC3\xA9", "a\xFFb".bytes, "\xC3" + "\xA9", ("\xC3" + "\xA9").force_encoding("UTF-8")
p [0xC3].pack("C").encoding, [233].pack("U").encoding, "é".unpack("a1"), "é".b.force_encoding("UTF-8"), "é".valid_encoding?
[-> { "\xC3".force_encoding("UTF-8") }, -> { "\xFF".b.force_encoding("US-ASCII") }].each do |f|
  begin
    p f.call
  rescue Encoding::InvalidByteSequenceError => e
    puts e.message
  end
end
`)
	assert.Equals(t, out, `#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
"café"
[97, 255, 98]
"\xC3\xA9"
"é"
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
["\xC3"]
"é"
true
invalid byte sequence in UTF-8: keep the bytes in a binary string
invalid byte sequence in US-ASCII: keep the bytes in a binary string
`)
}
