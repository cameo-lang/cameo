package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A binary string answers what reads, makes and compares bytes, and a
// pattern, as Ruby's does.
func TestABinaryStringAnswersItsBytes(t *testing.T) {
	out := runScript(t, `
b = b"héllo\xFF"
p b.length, b.size, b.bytesize, b[1], b[1, 2], b[-1], b.bytes.first(3), b.getbyte(0), b.empty?
p b + "x".b, b * 2 == b + b, b == b"héllo\xFF", b.index("l"), b.include?("ll"), b.start_with?("h"), b.end_with?(b"\xFF")
p b.delete_prefix("h"), b.unpack("C2"), b.encoding, b.ascii_only?, b.hash == b"héllo\xFF".hash, b.dup == b
p b.sub("l", "L"), b.gsub(/l/n, "L"), b.scan(/l/n).size, b =~ /l/n, b.match?(/\xFF/n)
`)
	assert.Equals(t, out, `7
7
7
"\xC3"
"\xC3\xA9"
"\xFF"
[104, 195, 169]
104
false
"h\xC3\xA9llo\xFFx"
true
true
3
true
true
true
"\xC3\xA9llo\xFF"
[104, 195]
#<Encoding:BINARY (ASCII-8BIT)>
false
true
true
"h\xC3\xA9Llo\xFF"
"h\xC3\xA9LLo\xFF"
2
3
true
`)
}

// A method that needs to know what character a byte is raises for a
// binary string, where Ruby reads the bytes as ASCII; the same bytes
// said to be UTF-8 answer it.
func TestABinaryStringHasNoCharacters(t *testing.T) {
	out := runScript(t, `
b = "caf\xC3\xA9 1".b
%w[upcase downcase capitalize swapcase chars each_char reverse strip lstrip chomp chop to_i to_f hex ord succ
   split lines squeeze to_sym codepoints].each do |name|
  begin
    b.send(name)
    puts "#{name} answered"
  rescue Encoding::CompatibilityError => e
    puts e.message unless e.message.start_with?("#{name} reads characters")
  end
end
[-> { b.center(9) }, -> { b.tr("a", "b") }, -> { b.count("a") }, -> { b % [] }, -> { b.casecmp("x") }].each do |f|
  begin
    f.call
    puts "answered"
  rescue Encoding::CompatibilityError
  end
end
t = b.force_encoding("UTF-8")
p t.upcase, t.chars.size, t.reverse, t.split, "abc".force_encoding("US-ASCII").upcase
`)
	assert.Equals(t, out, "\"CAFÉ 1\"\n6\n\"1 éfac\"\n[\"café\", \"1\"]\n\"ABC\"\n")
}
