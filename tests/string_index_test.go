package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// An Integer index reads one character without decoding the string: from
// the bytes of an ASCII, a US-ASCII or a binary string, and by walking
// from the nearer end of any other. The results are Ruby's.
func TestAnIntegerIndexReturnsTheCharacterThere(t *testing.T) {
	out := runScript(t, `s = "héllo wörld"
p s[0], s[1], s[2], s[9], s[10], s[11], s[-1], s[-2], s[-10], s[-11], s[-12]
a = "hello"
p a[0], a[4], a[5], a[-1], a[-5], a[-6]
p format("%s", "héllo")[1], "#{:héllo}!"[-1], [1, "é"].to_s[5]
bin = b"\xFFab"
p bin[0], bin[0].encoding, bin[-1], bin[3]
us = "abc".force_encoding("US-ASCII")
p us[1], us[1].encoding, us[-4]
p :héllo[1], "héllo"[1..2], "hello"[1, 3], "hello"[3..], "héllo"[-3, 2], ""[0]
`)
	assert.Equals(t, out, `"h"
"é"
"l"
"l"
"d"
nil
"d"
"l"
"é"
"h"
nil
"h"
"o"
nil
"o"
"h"
nil
"é"
"!"
"é"
"\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
"b"
nil
"b"
#<Encoding:US-ASCII>
nil
"é"
"él"
"ell"
"lo"
"ll"
nil
`)
}

// getbyte reads a byte of a string whose bytes are its characters, an
// ASCII one, or of a binary string. A UTF-8 string with characters beyond
// ASCII is refused, where Ruby returns a part of a character.
func TestGetbyteRefusesAStringWithCharactersBeyondASCII(t *testing.T) {
	out := runScript(t, `p "abc".getbyte(0), "abc".getbyte(-1), "abc".getbyte(3)
p "é".b.getbyte(0), b"\xFFa".getbyte(1), "abc".force_encoding("US-ASCII").getbyte(1)
p format("%s", "abc").getbyte(1), "#{:a}b".getbyte(1)
["é", "aé", format("%s", "é"), "#{:é}"].each do |s|
  begin
    s.getbyte(0)
  rescue Encoding::CompatibilityError => e
    puts e.message
  end
end
`)
	refusal := "getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with []\n"
	assert.Equals(t, out, "97\n99\nnil\n195\n97\n98\n98\n98\n"+refusal+refusal+refusal+refusal)
}

// Indexing does not depend on the length of the string: reading every
// character of a long ASCII string by index makes as many steps as it
// has characters. Before, each index decoded the whole string, and this
// script ran for minutes.
func TestIndexingALongStringIsNotQuadratic(t *testing.T) {
	out := runScript(t, `made = { "repeat" => "ab " * 100_000, "format" => format("%s", "ab " * 100_000), "interpolated" => "#{:x}#{"ab " * 100_000}", "binary" => ("ab " * 100_000).b }
made.each do |name, s|
  i = 0
  i += 1 until s[i].nil?
  j = 0
  j += 1 until s.getbyte(j).nil?
  puts "#{name}: #{i} #{j}"
end
`)
	assert.Equals(t, out, "repeat: 300000 300000\nformat: 300000 300000\ninterpolated: 300001 300001\nbinary: 300000 300000\n")
}

// A long string with characters beyond ASCII is found through an index of
// its characters. Every index gives the character chars gives, forwards,
// backwards, from the end and out of order, for several strings at once,
// for more strings than keep an index, and for slices that share bytes.
func TestTheIndexOfCharactersFindsEachCharacter(t *testing.T) {
	out := runScript(t, `s = "aé€😀b" * 300
chars = s.chars
n = chars.size
p n, s.bytesize
p (0...n).all? { |i| s[i] == chars[i] }
p (1..n).all? { |i| s[-i] == chars[n - i] }
p (0...n).to_a.reverse.all? { |i| s[i] == chars[i] }
p [1499, 0, 700, 64, 63, 65, 1280, 3, 1498].map { |i| s[i] == chars[i] }.all?
p s[n], s[n + 100], s[-n - 1], s[n - 1], s[-n]
others = (1..12).map { |k| "é" * (300 + k) + k.to_s }
p others.each_with_index.all? { |o, k| o[300 + k + 1] == (k + 1).to_s[0] && o[5] == "é" && s[k] == chars[k] }
head, tail = s[0, 600], s[600..]
p (0...600).all? { |i| head[i] == chars[i] }, (0...900).all? { |i| tail[i] == chars[600 + i] }, head[600], tail[900]
`)
	assert.Equals(t, out, "1500\n3300\ntrue\ntrue\ntrue\ntrue\nnil\nnil\nnil\n\"b\"\n\"a\"\ntrue\ntrue\ntrue\nnil\nnil\n")
}

// Reading a long string with characters beyond ASCII by index reads each
// character once: this loop made a walk from the start for each index,
// and took minutes at this length.
func TestIndexingALongStringOfCharactersIsNotQuadratic(t *testing.T) {
	out := runScript(t, `s = "é" + "ab " * 200_000
i = 0
i += 1 until s[i].nil?
p i, s[i - 1], s[0]
`)
	assert.Equals(t, out, "600001\n\" \"\n\"é\"\n")
}

// Workers index strings at once: the index is one for all of them.
func TestWorkersIndexStringsAtOnce(t *testing.T) {
	out := runScript(t, `include Cameo
class Reader
  def count(k)
    s = "é#{k}" + "aé" * 2_000
    n = 0
    i = 0
    until s[i].nil?
      n += 1 if s[i] == "é"
      i += 1
    end
    n
  end
end
r = Reader.new
p await((1..8).map { |k| async r, :count, k })
`)
	assert.Equals(t, out, "[2001, 2001, 2001, 2001, 2001, 2001, 2001, 2001]\n")
}
