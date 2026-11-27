# digest: testcase to demonstrate that digest objects are values
#
# A digest object is a value: feeding it returns a new one, and the one fed
# before is as it was, where Ruby feeds the object in place.
#
# The expected strings are Ruby 3.4's, with digest 3.2.0 and openssl 3.3.0.
#
# (Introduced in PR #267. expected output follows the __END__ separator.)
require "digest"
a = Digest::SHA256.new
b = a.update("abc")
p a.hexdigest == Digest::SHA256.hexdigest(""), b.hexdigest == Digest::SHA256.hexdigest("abc"), a == b
p b.hexdigest("x") == Digest::SHA256.hexdigest("x"), b.digest.bytesize, b.base64digest, (b << "d").hexdigest == Digest::SHA256.hexdigest("abcd")
begin; Digest::SHA256.file("x"); rescue NotImplementedError => e; puts e.message; end
begin; Digest::Class.new; rescue NotImplementedError => e; puts e.message; end
begin; Digest::SHA2.new(224); rescue ArgumentError => e; puts e.message; end
begin; Digest(:Nope); rescue LoadError => e; puts e.message; end
p require("digest/sha2"), require("digest"), require("openssl"), OpenSSL::Digest.hexdigest("sha1", "a")[0, 8]

__END__
true
true
false
true
32
"ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0="
true
Digest::SHA256.file reads a file, which cameo does not; digest the string instead (see https://15b.it/io)
Digest::Class is an abstract class
unsupported bit length: 224
library not found for class Digest::Nope -- digest/nope
false
false
true
"86f7e437"
