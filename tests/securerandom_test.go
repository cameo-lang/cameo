package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with securerandom 0.4.1: a seeded
// Random draws the values CRuby's does.

func TestRandomFormatterDrawsRubysValuesForASeed(t *testing.T) {
	out := runScript(t, `require "securerandom"
r = Random.new(42)
p r.hex(4), r.random_number(10), r.random_number(1..6), r.random_number(2.5).class, r.alphanumeric(5), r.base64(3), r.urlsafe_base64(4), r.urlsafe_base64(4, true), r.uuid.size, r.random_bytes(3).bytes, r.random_number(10**20).class
r = Random.new(9)
p Array.new(6) { r.random_number(100) }, r.random_number(0), r.random_number(-3).class, r.random_number(2...2), r.alphanumeric(3, chars: %w[x y z]), r.alphanumeric(40, chars: ("a".."h").to_a)
p Random.new(5).alphanumeric(70), Random.new(3).alphanumeric(8, chars: ["a", "b"]), Random.new(1).alphanumeric(3)
`)
	assert.Equals(t, out, `"66dce15f"
3
5
Float
"PQEnI"
"vK5B"
"FCzLmA"
"ZtbwJw=="
36
[202, 85, 145]
Integer
[92, 54, 56, 22, 65, 22]
0.8772212074585868
Float
0.13861327680989832
"xzz"
"ddfebfedadbeggcdafghfddffdefcfedhfeeegac"
"QrZCQRRuFjn9mnXo1UF8sLqDhe1jAxUkqGa7UvdgqUXoqxHqmEAOrQewt4VF59YlNhJ8i2"
"abababba"
"Ji6"
`)
}

func TestSecureRandomHasRubysShapes(t *testing.T) {
	out := runScript(t, `require "securerandom"
p SecureRandom.hex.size, SecureRandom.hex(4).size, SecureRandom.hex(nil).size, SecureRandom.uuid.size, SecureRandom.uuid_v7.size, SecureRandom.random_bytes.bytesize, SecureRandom.bytes(3).encoding
p SecureRandom.uuid[14], SecureRandom.uuid_v7[14], "89ab".include?(SecureRandom.uuid[19]), SecureRandom.uuid.count("-")
p SecureRandom.random_number.class, SecureRandom.random_number < 1, SecureRandom.random_number(6).between?(0, 5), SecureRandom.random_number(3..3), SecureRandom.random_number(1.0).class
p SecureRandom.base64.size, SecureRandom.base64(3).size, SecureRandom.urlsafe_base64.size, SecureRandom.urlsafe_base64(4, true).size, SecureRandom.alphanumeric.size
p SecureRandom::VERSION, Random::Formatter.class, Random.ancestors.include?(Random::Formatter), Random.hex(2).size, SecureRandom.gen_random(2).bytesize
begin; SecureRandom.hex(-1); rescue ArgumentError => e; p e.message; end
begin; SecureRandom.random_number("x"); rescue ArgumentError => e; p e.message; end
`)
	assert.Equals(t, out, `32
8
32
36
36
16
#<Encoding:BINARY (ASCII-8BIT)>
"4"
"7"
true
4
Float
true
true
3
Float
24
4
22
8
16
"0.4.1"
Module
true
4
2
"negative string size (or size too big)"
"invalid argument - String"
`)
}
