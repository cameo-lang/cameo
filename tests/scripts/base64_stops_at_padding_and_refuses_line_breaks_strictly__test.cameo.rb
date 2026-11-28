# base64: testcase to demonstrate that base64 stops at padding and refuses
# line breaks strictly
#
# A padding character that closes a group ends the decoding, as unpack("m")
# stops there; strict input may hold no line break; a keyword is one argument
# too many, but urlsafe_encode64's padding.
#
# The expected strings are Ruby 3.4.1's, with base64 0.2.0.
#
# (Introduced in PR #268. expected output follows the __END__ separator.)
require "base64"
p Base64.decode64("aGk=aGk="), Base64.decode64("aGk=xy"), Base64.decode64("aGVsbG8=d29ybGQ="), Base64.decode64(Base64.encode64("hi") + Base64.encode64("hi"))
p Base64.decode64("YWJj=ZGVm"), Base64.decode64("YWJjZ=GVm"), Base64.decode64("YWJjZG=Vm"), "aGk=aGk=".unpack1("m"), "a=Gk".unpack1("m")
["aGk=\n", "aG\nk=", "aGk=\r\n"].each { |s| begin; p Base64.strict_decode64(s); rescue ArgumentError => e; puts e.message; end }
begin; p Base64.urlsafe_decode64("aGk=\n\n\n"); rescue ArgumentError => e; puts e.message; end
begin; Base64.encode64("x", foo: 1); rescue ArgumentError => e; puts e.message; end
begin; Base64.urlsafe_encode64("x", foo: 1); rescue ArgumentError => e; puts e.message; end
begin; Base64.decode64("x", foo: 1); rescue ArgumentError => e; puts e.message; end

__END__
"hi"
"hi"
"hello"
"hi"
"abcdef"
"abcdef"
"abcd"
"hi"
"hi"
invalid base64
invalid base64
invalid base64
invalid base64
wrong number of arguments (given 2, expected 1)
unknown keyword: :foo
wrong number of arguments (given 2, expected 1)
