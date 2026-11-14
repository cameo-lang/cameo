package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Binary joined with anything is binary, whatever the parts hold: Ruby
// returns UTF-8 when the binary part is ASCII alone, and raises when both
// hold other bytes. A join of text alone stays text.
func TestBinaryJoinedWithAnythingIsBinary(t *testing.T) {
	out := runScript(t, `bin = b"ab"
hi = b"\xFF"
p ("x" + bin).encoding, (bin + "x").encoding, ("é" + hi).encoding, ("é" + hi).bytes, ("x" + b"").encoding
p [bin, "é"].join("-").encoding, ["a", "é"].join.encoding, ("x".force_encoding("US-ASCII") + bin).encoding
s = "x"
s <<= bin
t = "é"
t += hi
u = "é"
u = u.prepend(bin)
p s.encoding, t.encoding, u.encoding, ("a" + "é").encoding
`)
	const binary = "#<Encoding:BINARY (ASCII-8BIT)>\n"
	assert.Equals(t, out, binary+binary+binary+"[195, 169, 255]\n"+binary+
		binary+"#<Encoding:UTF-8>\n"+binary+
		binary+binary+binary+"#<Encoding:UTF-8>\n")
}

// An interpolation and format make text for a reader, and refuse a
// binary String, as the format or among the arguments, where Ruby makes
// a String of them. A String that says what its bytes are goes in.
func TestTextForAReaderRefusesABinaryString(t *testing.T) {
	out := runScript(t, `def try
  yield
rescue Encoding::CompatibilityError => e
  puts e.message
end
bin = b"ab"
try { "v=#{bin}" }
try { "v=#{b"\xFF"}" }
try { format("%s", bin) }
try { "%s" % bin }
try { "%s %s" % ["a", bin] }
try { format(bin, 1) }
try { sprintf("%d %s", 1, b"\xFF") }
p "v=#{bin.force_encoding("UTF-8")}", format("%s", "é"), "#{1}#{:a}#{nil}"
puts bin
`)
	const interpolation = "an interpolation makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with +\n"
	const format = "format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with +\n"
	assert.Equals(t, out, interpolation+interpolation+format+format+format+format+format+"\"v=ab\"\n\"é\"\n\"1a\"\nab\n")
}
