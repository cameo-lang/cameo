# binary paths: testcase to demonstrate that pack with u and bytes
#
# pack with U and another directive that puts in bytes which are no characters
# returns a binary String; with U alone a number that is no character is
# refused, and text stays text.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
p [233, 255].pack("UC"), [233, 255].pack("UC").encoding
p [233].pack("U"), [233].pack("U").encoding, [233, 97].pack("UC"), [233, 97].pack("UC").encoding

__END__
"\xC3\xA9\xFF"
#<Encoding:BINARY (ASCII-8BIT)>
"é"
#<Encoding:UTF-8>
"éa"
#<Encoding:UTF-8>
