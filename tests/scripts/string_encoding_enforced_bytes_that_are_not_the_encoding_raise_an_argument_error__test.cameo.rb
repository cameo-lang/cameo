# string encoding enforced: testcase to demonstrate that bytes that are not
# the encoding raise an argument error
#
# A String made as text of bytes that are not UTF-8 is refused where it is
# made, and the script gets an ArgumentError it can rescue: here a StringIO of
# text whose buffer was given a byte that is none.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
require "stringio"
io = StringIO.new("abc")
io.ungetbyte(255)
begin
  io.read
rescue ArgumentError => e
  puts e.message
end
puts "went on"

__END__
invalid byte sequence in UTF-8: "\xffabc" is not text; keep such bytes in a binary string (see https://15b.it/inv)
went on
