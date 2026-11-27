# stringio: testcase to demonstrate that string IO checks its reads and chomps
# as Ruby does
#
# The reads refuse a negative length and give "" for a zero one, pread checks
# its arguments and the mode, an unget past the end pads with NUL, a huge
# length is clamped, chomp takes "\r\n" off a default-separated line and
# nothing off an unseparated one, a limit of 0 reads "" from gets and is
# refused by each_line and readlines, a subclass's instances are the
# subclass's, and padding to a size cameo cannot hold is refused. The answers
# are Ruby 3.4.1's.
#
# The expected values are Ruby's, with stringio 3.1.2, but where Ruby's string
# returns the buffer itself, which later writes change, cameo's returns the
# String as it is then.
#
# (Introduced in PR #286. expected output follows the __END__ separator.)
require "stringio"
begin; StringIO.new("abc").readpartial(-1); rescue ArgumentError => e; puts e.message; end
begin; StringIO.new("abc").pread(2, -1); rescue Errno::EINVAL => e; puts e.message; end
begin; StringIO.new("abc").pread(-1, 1); rescue ArgumentError => e; puts e.message; end
begin; StringIO.new("abc", "a").pread(1, 0); rescue IOError => e; puts e.message; end
s = StringIO.new("abc"); s.seek(5); s.ungetc("x"); p s.string, s.pos
s = StringIO.new("abc"); s.seek(4); s.ungetc("x"); p s.string, s.pos
s = StringIO.new("abc"); s.read(1); p s.read(2**63 - 1)
p StringIO.new("a\r\nb").gets(chomp: true), StringIO.new("a\r\nb").readlines(chomp: true)
s = StringIO.new("ab\n"); p s.gets(0), s.pos
begin; StringIO.new("ab\n").readlines(0); rescue ArgumentError => e; puts e.message; end
s = StringIO.new("abc"); s.read; p s.readpartial(0), StringIO.new("abc").pread(0, 5)
begin; StringIO.new("abc").pread(1, 5); rescue EOFError => e; puts e.message; end
s = StringIO.new("abc"); s.seek(10); s.write("x"); p s.string.bytesize
p StringIO.new("a\n\n\nb").gets("", chomp: true), StringIO.new("axyb").gets("xy", chomp: true), StringIO.new("a\r\nb").gets(nil, chomp: true), StringIO.new("ab\n").gets(1, chomp: true)
class Capture < StringIO
  def hi
    1
  end
end
m = Capture.new("q"); p m.class, m.hi, m.read
begin; StringIO.new("abc").truncate(2**40); rescue ArgumentError => e; puts e.message; end

__END__
negative length -1 given
Invalid argument - pread: Invalid offset argument: -1
negative string size (or size too big): -1
not opened for reading
"abc\u0000x"
4
"abcx"
3
"bc"
"a"
["a", "b"]
""
0
invalid limit: 0 for readlines
""
""
end of file reached
11
"a"
"a"
"a\r\nb"
"a"
Capture
1
"q"
negative string size (or size too big)
