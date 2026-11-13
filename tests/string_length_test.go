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

// A string made by a method whose result has the characters of the
// receiver, or the receiver less some of one byte, by a substitution, a
// join, a symbol or a number knows its characters, and its length is
// what counting them one by one gives, as in Ruby.
func TestLengthOfStringsMadeByMethodsThatCount(t *testing.T) {
	out := runScript(t, `
a = "  Héllo Wörld\n"
half = "\xC3".force_encoding("UTF-8")
rest = "\xA9".force_encoding("UTF-8")
made = [
  a.upcase, a.downcase, a.capitalize, a.swapcase, a.reverse, a.strip, a.lstrip, a.rstrip, a.chomp, a.chop,
  a.to_s, a.to_str, "".chop, "é".chop, "".strip, (half * 2).reverse, half.chop,
  a.gsub("l", "łł"), a.gsub(/ö/) { |m| m * 3 }, a.sub("é", ""), a.gsub("z", "y"), half.gsub("x", "y"),
  [a, "é", 1, 2.5, :sým, nil, ["ü", 3]].join, [a, "é", 1].join("—"), [half, rest].join, [half, rest].join("-"),
  [].join, ["é"].join(","), [a.b, "x"].join, :sým.to_s, :abc.to_s, 12.to_s, 12.inspect, (2**80).to_s, 1.5.to_s,
]
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq
`)
	assert.Equals(t, out, "[14, 14, 14, 14, 14, 11, 12, 13, 13, 13, 14, 14, 0, 0, 0, 2, 0, 17, 16, 13, 14, 1, 24, 18, 1, 3, 0, 1, 17, 3, 3, 2, 2, 25, 3]\n[true]\n")
}

// A substitution in a string that knows its characters gives a string
// that knows its own, from what matched and what replaced it; one that
// cuts a character or puts half of one in is counted, and has the same
// length as in Ruby.
func TestLengthOfSubstitutedStrings(t *testing.T) {
	out := runScript(t, `
a = "  Héllo Wörld\n"
half = "\xC3".force_encoding("UTF-8")
rest = "\xA9".force_encoding("UTF-8")
made = [
  a.gsub("l", "łł"), a.gsub(/ö/) { |m| m * 3 }, a.sub("é", ""), a.gsub("z", "y"), half.gsub("x", "y"),
  a.gsub("", "-"), a.gsub(/(l+)/, '<\1é>'), a.gsub(/[éö]/, "é" => "e", "ö" => "öö"), a.sub(/\s+/, ""),
  (half + "x").sub("x", rest), (half + "x" + rest).sub("x", ""), "aéb".gsub(/b|a/, half), a.gsub(/./, ""),
  a.b.gsub("l", "L").force_encoding("UTF-8"), a.gsub("é", "e").gsub("ö", "o"), "".gsub("", "é"),
]
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq
`)
	assert.Equals(t, out, "[17, 16, 13, 14, 1, 29, 20, 15, 12, 1, 1, 3, 1, 14, 14, 1]\n[true]\n")
}

// tr and squeeze count the characters they write, padding and the
// removal of a prefix or a suffix have theirs from the receiver and the
// argument, and what split, scan and CSV cut out of ASCII is ASCII:
// each length is what counting one by one gives, as in Ruby.
func TestLengthOfStringsWhoseCountFollowsFromTheirMaking(t *testing.T) {
	out := runScript(t, `
require "csv"
a = "  Héllo Wörld\n"
x = "hello, world  foo"
half = "\xC3".force_encoding("UTF-8")
rest = "\xA9".force_encoding("UTF-8")
made = [
  a.tr("lo", "łø"), a.tr("l", ""), a.tr("^l", "é"), a.tr("a-z", "A-Z"), x.tr("o", "ö"),
  a.squeeze, "aaééé  b".squeeze, "".squeeze, x.squeeze,
  a.center(20), a.center(3), a.ljust(20, "é"), a.rjust(21, "éø"), x.center(30, "*"), "".rjust(3, "é"),
  rest.rjust(3, "é"), rest.ljust(3, "é"), (half + "a").center(6, "ö"),
  a.chomp("\n"), a.chomp("ld\n"), a.chomp("x"), a.chomp(""), "é\r\n\n".chomp(""), (half + rest).chomp(rest), x.chomp("foo"),
  a.delete_prefix("  H"), a.delete_prefix("x"), a.delete_suffix("rld\n"), "éa".delete_prefix("é"), (half + rest).delete_prefix(half),
  "é".delete_prefix("é"), "é".delete_suffix("é"),
]
made += a.split + a.split("l") + x.split(", ") + x.split("") + a.split("") + x.split(/o/, 2) + a.split(/(ö)/)
made += x.scan(/o./) + a.scan(/[éö]./) + x.scan(/(l)(.)/).flatten + a.scan(/(l)(.)/).flatten + x.scan("o")
made += CSV.parse("a,b c,\"d,e\"\n1,,3\n").flatten.compact + CSV.parse("é,b\nc,ö\n").flatten + CSV.parse_line("x,y")
p made.map { |s| s.length }
p made.map { |s| s.length == s.each_char.count }.uniq
`)
	assert.Equals(t, out, "[14, 11, 14, 14, 17, 12, 4, 0, 14, 20, 14, 20, 21, 30, 3, 3, 3, 6, 13, 11, 14, 13, 1, 1, 14, 11, 14, 10, 1, 1, 0, 0, 5, 5, 4, 0, 5, 2, 5, 10, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 4, 12, 9, 1, 4, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 3, 1, 1, 1, 1, 1, 1, 1, 1]\n[true]\n")
}
