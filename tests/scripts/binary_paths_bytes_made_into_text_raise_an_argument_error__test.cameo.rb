# binary paths: testcase to demonstrate that bytes made into text raise an
# ArgumentError a script can rescue
#
# A String made as text of bytes that are none, where no method has an error
# of its own, raises an ArgumentError with a message that says what the bytes
# are: here a StringIO of text read from the middle of a character. The line
# the error names is checked in tests/binary_paths_test.go.
#
# Each test here is a path that handles a binary String, or bytes that are not
# UTF-8, and raised when Strings began to be checked where they are made: a
# review found them.
#
# (Introduced in PR #438. expected output follows the __END__ separator.)
require "stringio"
io = StringIO.new("héllo")
io.read(2)
begin
  io.gets
rescue ArgumentError => e
  puts e.message
end
puts "went on"

__END__
invalid byte sequence in UTF-8: "\xa9llo" is not text; keep such bytes in a binary string (see https://15b.it/inv)
went on
