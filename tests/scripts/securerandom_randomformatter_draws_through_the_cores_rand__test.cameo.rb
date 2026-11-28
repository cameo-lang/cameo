# securerandom: testcase to demonstrate that random formatter draws through
# the core's rand
#
# The numbers are stdlib's draws, Random#rand's, over a source: Random's own
# class methods stay the core's, a seeded Random draws Ruby's values through
# the formatter too, SecureRandom takes any limit a Random takes, and a class
# that includes Random::Formatter draws from its bytes as Ruby's module does,
# with its error as the result.
#
# The expected strings are Ruby 3.4's, with securerandom 0.4.1: a seeded
# Random draws the values CRuby's does.
#
# (Introduced in PR #269. expected output follows the __END__ separator.)
require "securerandom"
srand(42); p Random.random_number(1.0..2.0); srand(42); p Random.rand(1.0..2.0)
p Random.new(42).random_number(10**20), Random.new(42).rand(10**20), Random.new(42).alphanumeric(4)
begin; Random.rand(0); rescue ArgumentError => e; puts e.message; end
p Random.random_number(0).class, Random.bytes(3).bytesize
p SecureRandom.random_number(2...2).class, SecureRandom.random_number(1.0...1.0).class, SecureRandom.random_number(10**20).class, SecureRandom.random_number(-2**63..2**63-1).class
begin; SecureRandom.hex(2**40); rescue ArgumentError => e; puts e.message; end
class Mine
  include Random::Formatter
  def bytes(n)
    "\x01".b * n
  end
end
m = Mine.new
p m.hex(2), m.random_number(10), m.alphanumeric(4), m.random_number(1.0..2.0), m.uuid
p m.random_number(10**20), m.rand(1000), m.random_number(5..9), m.base64(3), m.random_number(3.0), m.rand
class Bad
  include Random::Formatter
  def bytes(n)
    raise "no bytes"
  end
end
begin; Bad.new.hex; rescue RuntimeError => e; puts e.message; end

__END__
1.3745401144950984
1.3745401144950984
72877815814522508558
72877815814522508558
"SyWM"
invalid argument - 0
Float
3
Float
Float
Integer
Integer
negative string size (or size too big)
"0101"
1
"bnpI"
1.003921568627451
"01010101-0101-4101-8101-010101010101"
18519084246547628289
257
6
"AQEB"
0.01176470526877571
0.003921568422925237
no bytes
