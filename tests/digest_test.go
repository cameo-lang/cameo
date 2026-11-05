package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with digest 3.2.0 and openssl 3.3.0.

func TestDigestOpenSSLNamesAndHMAC(t *testing.T) {
	out := runScript(t, `require "digest"
p OpenSSL::HMAC.hexdigest("SHA256", "key", "The quick brown fox jumps over the lazy dog")
p OpenSSL::HMAC.hexdigest("sha1", "key", "data"), OpenSSL::HMAC.base64digest("SHA256", "k", "d"), OpenSSL::HMAC.digest("MD5", "k", "d").bytesize
p OpenSSL::Digest::SHA256.hexdigest("abc") == Digest::SHA256.hexdigest("abc"), OpenSSL::Digest.new("SHA256").update("abc").hexdigest, OpenSSL::Digest.hexdigest("sha256", "abc")[0, 8]
p OpenSSL::Digest.new("SHA-512", "abc").hexdigest[0, 8], OpenSSL::Digest::MD5.hexdigest("")
begin; OpenSSL::Digest.new("SHA3-256"); rescue RuntimeError => e; puts e.message; end
`)
	assert.Equals(t, out, `"f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
"104152c5bfdca07bc633eebd46199f0255c9f49d"
"5+ohw7y2Ok2jrXhQMWjTa9ygvmIjgupgoQj61OSWZnk="
16
true
"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
"ba7816bf"
"ddaf35a1"
"d41d8cd98f00b204e9800998ecf8427e"
Unsupported digest algorithm (SHA3-256).: unknown object name
`)
}

// A digest object is a value: feeding it returns a new one, and the one
// fed before is as it was, where Ruby feeds the object in place.
func TestDigestObjectsAreValues(t *testing.T) {
	out := runScript(t, `require "digest"
a = Digest::SHA256.new
b = a.update("abc")
p a.hexdigest == Digest::SHA256.hexdigest(""), b.hexdigest == Digest::SHA256.hexdigest("abc"), a == b
p b.hexdigest("x") == Digest::SHA256.hexdigest("x"), b.digest.bytesize, b.base64digest, (b << "d").hexdigest == Digest::SHA256.hexdigest("abcd")
begin; Digest::SHA256.file("x"); rescue NotImplementedError => e; puts e.message; end
begin; Digest::Class.new; rescue NotImplementedError => e; puts e.message; end
begin; Digest::SHA2.new(224); rescue ArgumentError => e; puts e.message; end
begin; Digest(:Nope); rescue LoadError => e; puts e.message; end
p require("digest/sha2"), require("digest"), require("openssl"), OpenSSL::Digest.hexdigest("sha1", "a")[0, 8]
`)
	assert.Equals(t, out, `true
true
false
true
32
"ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0="
true
Digest::SHA256.file reads a file, which cameo does not; digest the string instead
Digest::Class is an abstract class
unsupported bit length: 224
library not found for class Digest::Nope -- digest/nope
false
false
true
"86f7e437"
`)
}

// The algorithm is found through a script's subclass, an OpenSSL method
// takes it as a digest object or class, a digest object is fed
// incrementally without holding its input, and SHA2's bit length is new's
// argument alone, as Ruby's arities say.
func TestDigestSubclassesIncrementsAndOpenSSLObjects(t *testing.T) {
	out := runScript(t, `require "digest"
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
`)
	assert.Equals(t, out, `"900150983cd24fb0d6963f7d28e17f72"
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
`)
}
