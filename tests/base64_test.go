package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with base64 0.2.0.

func TestBase64DecodesLenientlyAsRubysUnpackDoes(t *testing.T) {
	out := runScript(t, `require "base64"
p Base64.decode64("a=Gk"), Base64.decode64("aG=k="), Base64.decode64("aGVsbG8!@#"), Base64.decode64("a"), Base64.decode64("aG"), Base64.decode64("aGVs bG8="), Base64.decode64("====aGk")
p Base64.urlsafe_decode64("aGk"), Base64.strict_decode64(""), Base64.encode64("a" * 45).count("\n"), Base64.encode64("a" * 46)
["aGk===", "aGk=x"].each { |s| begin; p Base64.urlsafe_decode64(s); rescue ArgumentError => e; p e.message; end }
p Base64.urlsafe_encode64("hi", padding: false), Base64.urlsafe_decode64("aGk").encoding, Base64.decode64("=").bytesize
p Base64.urlsafe_decode64("a+b/"), Base64.urlsafe_decode64("a-b_")
`)
	assert.Equals(t, out, `"hi"
"h"
"hello"
""
"h"
"hello"
"hi"
"hi"
""
1
"YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFh\nYQ==\n"
"invalid base64"
"invalid base64"
"aGk"
#<Encoding:BINARY (ASCII-8BIT)>
0
"k\xE6\xFF"
"k\xE6\xFF"
`)
}
