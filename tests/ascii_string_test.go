package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A US-ASCII string holds 7-bit characters alone. Bytes past them are
// refused where a script says they are US-ASCII, unlike in Ruby, and a
// method that brings other characters in through an argument gives a
// UTF-8 string, as in Ruby.
func TestAnASCIIStringHoldsSevenBitCharactersAlone(t *testing.T) {
	out := runScript(t, `
a = "abc".force_encoding("ASCII")
p a.encoding, a.length, a.upcase, a.upcase.encoding, (a + "é").encoding, Encoding::US_ASCII
begin
  "é".force_encoding("ASCII")
rescue Encoding::InvalidByteSequenceError => e
  puts e.message
end
tests = {
  "tr" => -> { a.tr("a", "é") }, "center" => -> { a.center(7, "é") }, "sub" => -> { a.sub("b", "é") },
  "gsub" => -> { a.gsub(/b/, "é") }, "*" => -> { a * 2 }, "ljust" => -> { a.ljust(5, "ü") },
  "interp" => -> { "#{a}é" }, "join" => -> { [a, "é"].join }, "%" => -> { a % [] },
  "delete_suffix" => -> { a.delete_suffix("é") }, "encode" => -> { "é".encode("US-ASCII") },
}
tests.each do |name, f|
  begin
    v = f.call
    puts "#{name}: #{v.inspect} #{v.encoding} valid=#{v.valid_encoding?}"
  rescue => e
    puts "#{name}: #{e.class}: #{e.message}"
  end
end
`)
	assert.Equals(t, out, "#<Encoding:US-ASCII>\n3\n\"ABC\"\n#<Encoding:US-ASCII>\n#<Encoding:UTF-8>\n#<Encoding:US-ASCII>\ninvalid byte sequence in US-ASCII: keep the bytes in a binary string\ntr: \"ébc\" UTF-8 valid=true\ncenter: \"ééabcéé\" UTF-8 valid=true\nsub: \"aéc\" UTF-8 valid=true\ngsub: \"aéc\" UTF-8 valid=true\n*: \"abcabc\" US-ASCII valid=true\nljust: \"abcüü\" UTF-8 valid=true\ninterp: \"abcé\" UTF-8 valid=true\njoin: \"abcé\" UTF-8 valid=true\n%: \"abc\" US-ASCII valid=true\ndelete_suffix: \"abc\" US-ASCII valid=true\nencode: Encoding::UndefinedConversionError: U+00E9 from UTF-8 to US-ASCII\n")
}
