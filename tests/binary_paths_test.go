package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// Each test here is a path that handles a binary String, or bytes that
// are not UTF-8, and raised when Strings began to be checked where they
// are made: a review found them.

// to_s, to_str and String() of a binary String are the String, in its
// encoding.
func TestABinaryStringIsItselfAsAString(t *testing.T) {
	out := runScript(t, `x = b"\xFFab"
p x.to_s, x.to_str, String(x), [x].map(&:to_s)
p x.to_s.encoding, String(x).encoding, String("é").encoding, String(12), String(:a)
`)
	assert.Equals(t, out, `"\xFFab"
"\xFFab"
"\xFFab"
["\xFFab"]
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
"12"
"a"
`)
}

// What a pattern finds in a binary String is bytes: the parts of a
// match, of partition and rpartition, what scan returns, and a
// MatchData's own methods.
func TestAMatchInABinaryStringIsBytes(t *testing.T) {
	out := runScript(t, `x = b"\xFFab"
m = x.match(/a/)
p m.pre_match, m.post_match, m[0], m.string, m.to_a
p [m.pre_match, m.post_match, m[0], m.string].map(&:encoding).uniq
p x.partition("a"), x.partition(/a/), x.rpartition("a"), x.rpartition(/a/)
p x.partition("a").map(&:encoding).uniq, x.scan(/./n), x.scan(/./n).map(&:encoding).uniq
p x.match(/(?<x>\xFF)/n).named_captures, x.match(/(a)(b)/).captures
p(/a/.match(x).pre_match, x =~ /b/)
t = "héllo".match(/l+/)
p t.pre_match, t.post_match, t[0], t.pre_match.encoding
`)
	assert.Equals(t, out, `"\xFF"
"b"
"a"
"\xFFab"
["a"]
[#<Encoding:BINARY (ASCII-8BIT)>]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
["\xFF", "a", "b"]
[#<Encoding:BINARY (ASCII-8BIT)>]
["\xFF", "a", "b"]
[#<Encoding:BINARY (ASCII-8BIT)>]
{"x" => "\xFF"}
["a", "b"]
"\xFF"
2
"hé"
"o"
"ll"
#<Encoding:UTF-8>
`)
}

// An exception's message is text for a reader, whatever bytes it was
// given: a byte that is part of no character is written as \xNN. Reading
// it in a rescue raises nothing.
func TestAnExceptionMessageWithBytesIsReadable(t *testing.T) {
	out := runScript(t, `begin
  raise ArgumentError, b"\xFFa"
rescue => e
  p e.message, e.message.encoding, e.to_s, e.inspect, e.full_message
end
begin
  nil.send(b"\xFFa")
rescue NoMethodError => e
  p e.message
end
begin
  raise "héllo"
rescue => e
  p e.message
end
`)
	assert.Equals(t, out, `"\\xFFa"
#<Encoding:UTF-8>
"\\xFFa"
"#<ArgumentError: \\xFFa>"
"\\xFFa (ArgumentError)"
"undefined method '\\xFFa' for nil"
"héllo"
`)
}

// What a block or a hash gives sub and gsub to put in is checked as an
// argument is: bytes that are no characters do not go into text, so no
// UTF-8 String is made of them. ASCII bytes are text.
func TestSubstitutionTakesNoBytesFromABlockOrAHash(t *testing.T) {
	out := runScript(t, `def try
  yield
rescue Encoding::CompatibilityError => e
  puts e.message
end
try { "abc".gsub("b") { b"\xFF" } }
try { "abc".gsub("b", { "b" => b"\xFF" }) }
try { "abc".sub(/b/) { b"\xFF" } }
s = "abc".gsub("b") { b"x" }
p s, s.encoding, s.valid_encoding?, s.length
r = b"a\xFFc".gsub(b"\xFF") { b"\xFE" }
p r, r.encoding
`)
	const no = "incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)\n"
	assert.Equals(t, out, no+no+no+`"axc"
#<Encoding:UTF-8>
true
3
"a\xFEc"
#<Encoding:BINARY (ASCII-8BIT)>
`)
}

// A StringScanner over a binary String gives bytes, and one over text
// gives text.
func TestAScannerGivesWhatItsStringIs(t *testing.T) {
	out := runScript(t, `require "strscan"
sc = StringScanner.new(b"\xFFab")
p sc.scan(/./n), sc.peek(1), sc.rest, sc.string, sc.getch, sc.pre_match, sc.post_match
p [sc.rest, sc.string, sc.matched].map(&:encoding).uniq
tx = StringScanner.new("héllo")
p tx.scan(/h./), tx.rest, tx.rest.encoding
tx.string = b"\xFEz"
p tx.scan(/./n), tx.rest.encoding
`)
	assert.Equals(t, out, `"\xFF"
"a"
"ab"
"\xFFab"
"a"
"\xFF"
"b"
[#<Encoding:BINARY (ASCII-8BIT)>]
"hé"
"llo"
#<Encoding:UTF-8>
"\xFE"
#<Encoding:BINARY (ASCII-8BIT)>
`)
}

// pack with U and another directive that puts in bytes which are no
// characters returns a binary String; with U alone a number that is no
// character is refused, and text stays text.
func TestPackWithUAndBytes(t *testing.T) {
	out := runScript(t, `p [233, 255].pack("UC"), [233, 255].pack("UC").encoding
p [233].pack("U"), [233].pack("U").encoding, [233, 97].pack("UC"), [233, 97].pack("UC").encoding
`)
	assert.Equals(t, out, `"\xC3\xA9\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
"é"
#<Encoding:UTF-8>
"éa"
#<Encoding:UTF-8>
`)
}

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
	const message = `invalid byte sequence in UTF-8: "\xa9llo" is not text; keep such bytes in a binary string`
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", script, &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:4: "+message+" (ArgumentError)")

	out := runScript(t, script[:len(script)-len("io.gets\n")]+`begin
  io.gets
rescue ArgumentError => e
  puts e.message
end
puts "went on"
`)
	assert.Equals(t, out, message+"\nwent on\n")
}
