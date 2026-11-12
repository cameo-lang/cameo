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
