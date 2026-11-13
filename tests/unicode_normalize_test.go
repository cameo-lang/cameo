package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's.

func TestUnicodeNormalizeFollowsRubysForms(t *testing.T) {
	out := runScript(t, `s = "é é ﬁ ① Å Å"
p [s.unicode_normalize, s.unicode_normalize(:nfc), s.unicode_normalize(:nfd), s.unicode_normalize(:nfkc), s.unicode_normalize(:nfkd)].map { |x| x.codepoints.map { |c| c.to_s(16) } }
p s.unicode_normalized?, s.unicode_normalized?(:nfd), "abc".unicode_normalized?, "é".unicode_normalized?(:nfc), "é".unicode_normalized?(:nfd), "".unicode_normalized?
p "한글".unicode_normalize(:nfd).codepoints.map { |c| c.to_s(16) }, "ẛ̣".unicode_normalize(:nfkc).codepoints.map { |c| c.to_s(16) }, "é".unicode_normalize(:nfd).length
p "ascii".force_encoding("US-ASCII").unicode_normalize.encoding, require("unicode_normalize")
begin; "x".unicode_normalize(:nfx); rescue ArgumentError => e; puts e.message; end
begin; "x".unicode_normalize("nfc"); rescue ArgumentError => e; puts e.message; end
begin; "x".b.unicode_normalize; rescue Encoding::CompatibilityError => e; puts e.message; end
begin; b"\xff".unicode_normalized?; rescue Encoding::CompatibilityError => e; puts e.message; end
`)
	assert.Equals(t, out, `[["e9", "20", "e9", "20", "fb01", "20", "2460", "20", "c5", "20", "c5"], ["e9", "20", "e9", "20", "fb01", "20", "2460", "20", "c5", "20", "c5"], ["65", "301", "20", "65", "301", "20", "fb01", "20", "2460", "20", "41", "30a", "20", "41", "30a"], ["e9", "20", "e9", "20", "66", "69", "20", "31", "20", "c5", "20", "c5"], ["65", "301", "20", "65", "301", "20", "66", "69", "20", "31", "20", "41", "30a", "20", "41", "30a"]]
false
false
true
true
true
true
["1112", "1161", "11ab", "1100", "1173", "11af"]
["1e69"]
2
#<Encoding:US-ASCII>
true
Invalid normalization form nfx.
Invalid normalization form nfc.
Unicode Normalization not appropriate for ASCII-8BIT
Unicode Normalization not appropriate for ASCII-8BIT
`)
}

// The encoding rules are Ruby's: bytes that are not UTF-8, which are a
// binary string, are refused, a US-ASCII one is as it is and normalized, a binary
// one is refused after the arguments are counted.
func TestUnicodeNormalizeKeepsRubysEncodingRules(t *testing.T) {
	out := runScript(t, `begin; b"\xff".unicode_normalize; rescue Encoding::CompatibilityError => e; puts e.message; end
begin; b"\xff".unicode_normalized?; rescue Encoding::CompatibilityError => e; puts e.message; end
p "e".force_encoding("US-ASCII").unicode_normalize(:nfd).bytes, "e".force_encoding("US-ASCII").unicode_normalized?(:nfd)
begin; "x".b.unicode_normalize(:nfc, :nfd); rescue ArgumentError => e; puts e.message; end
begin; "x".b.unicode_normalize; rescue Encoding::CompatibilityError => e; puts e.message; end
`)
	assert.Equals(t, out, `Unicode Normalization not appropriate for ASCII-8BIT
Unicode Normalization not appropriate for ASCII-8BIT
[101]
true
wrong number of arguments (given 2, expected 0..1)
Unicode Normalization not appropriate for ASCII-8BIT
`)
}
