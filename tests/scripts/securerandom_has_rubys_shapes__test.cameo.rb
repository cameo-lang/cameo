# securerandom: testcase to demonstrate that secure random has Ruby's shapes
#
# The expected strings are Ruby 3.4's, with securerandom 0.4.1: a seeded
# Random draws the values CRuby's does.
#
# (Introduced in PR #269. expected output follows the __END__ separator.)
require "securerandom"
p SecureRandom.hex.size, SecureRandom.hex(4).size, SecureRandom.hex(nil).size, SecureRandom.uuid.size, SecureRandom.uuid_v7.size, SecureRandom.random_bytes.bytesize, SecureRandom.bytes(3).encoding
p SecureRandom.uuid[14], SecureRandom.uuid_v7[14], "89ab".include?(SecureRandom.uuid[19]), SecureRandom.uuid.count("-")
p SecureRandom.random_number.class, SecureRandom.random_number < 1, SecureRandom.random_number(6).between?(0, 5), SecureRandom.random_number(3..3), SecureRandom.random_number(1.0).class
p SecureRandom.base64.size, SecureRandom.base64(3).size, SecureRandom.urlsafe_base64.size, SecureRandom.urlsafe_base64(4, true).size, SecureRandom.alphanumeric.size
p SecureRandom::VERSION, Random::Formatter.class, Random.ancestors.include?(Random::Formatter), Random.hex(2).size, SecureRandom.gen_random(2).bytesize
begin; SecureRandom.hex(-1); rescue ArgumentError => e; p e.message; end
begin; SecureRandom.random_number("x"); rescue ArgumentError => e; p e.message; end

__END__
32
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
"invalid argument - x"
