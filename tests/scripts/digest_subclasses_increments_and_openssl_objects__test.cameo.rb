# digest: testcase to demonstrate that digest subclasses increments and open
# SSL objects
#
# The algorithm is found through a script's subclass, an OpenSSL method takes
# it as a digest object or class, a digest object is fed incrementally without
# holding its input, and SHA2's bit length is new's argument alone, as Ruby's
# arities say.
#
# The expected strings are Ruby 3.4's, with digest 3.2.0 and openssl 3.3.0.
#
# (Introduced in PR #267. expected output follows the __END__ separator.)
require "digest"
class MyMD5 < Digest::MD5; end
p MyMD5.new.update("abc").hexdigest
p OpenSSL::HMAC.hexdigest(OpenSSL::Digest.new("sha256"), "k", "d") == OpenSSL::HMAC.hexdigest("sha256", "k", "d"), OpenSSL::HMAC.hexdigest(OpenSSL::Digest::SHA256, "k", "d")[0, 16]
p OpenSSL::Digest.new("sha256").name, Digest::SHA2.new(384).name, OpenSSL::Digest.hexdigest(OpenSSL::Digest.new("sha1"), "x")[0, 8]
d = 1000.times.reduce(Digest::SHA512.new) { |h, _| h << "chunk" }
p d.hexdigest == Digest::SHA512.hexdigest("chunk" * 1000), Digest::SHA2.new(384).update("abc") == Digest::SHA384.new.update("abc")
begin; Digest::SHA2.new(384, "x"); rescue ArgumentError => e; puts e.message; end
begin; Digest::SHA256.new("x"); rescue ArgumentError => e; puts e.message; end
begin; Digest::SHA2.hexdigest(384, "abc"); rescue ArgumentError => e; puts e.message; end
begin; Digest::SHA2.hexdigest(1); rescue TypeError => e; puts e.message; end
begin; Digest::SHA2.new(100); rescue ArgumentError => e; puts e.message; end

__END__
"900150983cd24fb0d6963f7d28e17f72"
true
"e7ea21c3bcb63a4d"
"SHA256"
"SHA384"
"11f6ad8e"
true
true
wrong number of arguments (given 2, expected 0..1)
wrong number of arguments (given 1, expected 0)
wrong number of arguments (given 2, expected 1)
no implicit conversion of Integer into String
unsupported bit length: 100
