package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A string's length is its characters, whether the string is known to
// be ASCII, a literal, a repetition or a sum of such, or is not.
func TestLengthOfStringsKnownToBeASCIIAndNot(t *testing.T) {
	out := runScript(t, `
p "abc".length, ("ab" * 3).size, ("é" * 3).length, ("a" + "é").length, ("a" + "b").length
p "".length, ("" * 5).length, "é".b.length, ("x" * 3 + "é").size, ("é" + "x" * 3).size
p ("a" * 2).force_encoding("ASCII-8BIT").length, "añb".size, ("a" * 3).encoding, ("a" + "b").encoding
s = "ab" * 2
p s == "abab", s.eql?("abab"), { s => 1 }["abab"], s.hash == "abab".hash
`)
	assert.Equals(t, out, "3\n6\n3\n2\n2\n0\n0\n2\n4\n4\n2\n3\n#<Encoding:UTF-8>\n#<Encoding:UTF-8>\ntrue\ntrue\n1\ntrue\n")
}

// A string made of strings that know their characters knows its own: a
// sum, a repetition, an interpolation, a part of a string, a character,
// a number's text. One whose bytes were relabelled or put together from
// halves of characters is counted, and has the same length as in Ruby.
func TestLengthOfStringsMadeOfCountedStrings(t *testing.T) {
	out := runScript(t, `
a = "héllo"
b = "wörld"
n = 12
p (a + b).length, (a * 3).length, "#{a} #{b}".length, "#{a}#{n}#{1.5}#{:sým}#{nil}".length
p a[1, 3].length, a[1..].length, a[1].length, a[-1].length, a.chars.map(&:length), a.chars.join.length
p n.to_s.length, 1.5.to_s.length, (a + n.to_s).length, "#{a}".length, "#{n}".length
half = "\xC3".force_encoding("UTF-8")
rest = "\xA9".force_encoding("UTF-8")
p half.length, rest.length, (half + rest).length, (half + "a").length, ("a" + rest).length, "#{half}#{rest}".length
p (rest * 3).length, (half * 2).length, a.b.length, a.b.force_encoding("UTF-8").length, (a.b + "x").length
p "#{a.b}x".length, a.dup.force_encoding("ASCII-8BIT").length
`)
	assert.Equals(t, out, "10\n15\n11\n13\n3\n4\n1\n1\n[1, 1, 1, 1, 1]\n5\n2\n3\n7\n5\n2\n1\n1\n1\n2\n2\n1\n3\n2\n6\n5\n7\n7\n6\n")
}
