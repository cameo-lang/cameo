# securerandom: testcase to demonstrate that random formatter draws Ruby's
# values for a seed
#
# The expected strings are Ruby 3.4's, with securerandom 0.4.1: a seeded
# Random draws the values CRuby's does.
#
# (Introduced in PR #269. expected output follows the __END__ separator.)
require "securerandom"
r = Random.new(42)
p r.hex(4), r.random_number(10), r.random_number(1..6), r.random_number(2.5).class, r.alphanumeric(5), r.base64(3), r.urlsafe_base64(4), r.urlsafe_base64(4, true), r.uuid.size, r.random_bytes(3).bytes, r.random_number(10**20).class
r = Random.new(9)
p Array.new(6) { r.random_number(100) }, r.random_number(0), r.random_number(-3).class, r.random_number(2...2), r.alphanumeric(3, chars: %w[x y z]), r.alphanumeric(40, chars: ("a".."h").to_a)
p Random.new(5).alphanumeric(70), Random.new(3).alphanumeric(8, chars: ["a", "b"]), Random.new(1).alphanumeric(3)

__END__
"66dce15f"
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
