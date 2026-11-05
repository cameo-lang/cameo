package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby's, with stringio 3.1.2, but where Ruby's
// string returns the buffer itself, which later writes change, cameo's
// returns the String as it is then.

func TestStringIOReadsWritesAndSeeksAsRubysDoes(t *testing.T) {
	out := runScript(t, `require "stringio"
io = StringIO.new("line1\nline2\nlast")
p io.read(3), io.pos, io.read, io.read, io.read(2), io.eof?, io.pos, io.size, io.string, io.lineno
io.rewind
p io.gets, io.lineno, io.gets(chomp: true), io.gets, io.gets, io.eof?
io.rewind
p io.readlines, io.rewind, io.readlines(chomp: true), io.rewind, io.each_line.to_a, io.rewind, io.each_line(chomp: true).map(&:upcase), io.rewind, io.map(&:size)
io.rewind
p io.getc, io.getc, io.readchar, io.getbyte, io.pos, io.ungetc("Z"), io.pos, io.read(3), io.each_char.first(2), io.pos, io.string
io.rewind
p io.gets("2"), io.gets(3), io.gets("", chomp: true), io.gets(nil), io.rewind, io.read(0), io.read(100), io.read(1), io.read(0), io.read
io.rewind
io.pos = 6
p io.read(5), io.seek(0), io.read(5), io.seek(-4, IO::SEEK_END), io.read, io.seek(2, IO::SEEK_SET), io.seek(1, IO::SEEK_CUR), io.read(2), io.tell, io.rewind, io.each_byte.first(3)
p StringIO.new("a\n\nb\n\n\nc").each_line("").to_a, StringIO.new("abc").each_char.to_a, StringIO.new("").readlines, StringIO.new("").gets, StringIO.new("").read, StringIO.new("").read(1), StringIO.new("").getc
w = StringIO.new
p w.write("hello"), w.write(" ", "world", 1), w.string, w.pos, (w << "!" << "?").string, (w.puts "a", ["b", "c"]), w.print("x", 1, nil), w.printf("%03d|%s", 7, "s"), w.string
s = StringIO.new("hello world")
s.pos = 6
s.write("there")
p s.string, s.pos, s.write("!!"), s.string, s.rewind, s.write("J"), s.string, s.read
s = StringIO.new("hello")
p s.truncate(2), s.string, s.truncate(4), s.string, s.size
s = StringIO.new("hello")
s.read(2)
p s.puts("X"), s.string, s.pos
s = StringIO.new("abc", "a")
s.write("d")
p s.string, s.pos
s = StringIO.new("abc", "w")
p s.string, s.write("x"), s.string
s = StringIO.new
s.puts
s.puts(nil)
s.puts([])
s.puts("a\n")
s.print
s.print("b")
p s.string
s = StringIO.new("ab")
s.seek(5)
s.write("x")
p s.string.bytes
s = StringIO.new("x\ny")
s.gets
s.lineno = 10
s.gets
p s.lineno, StringIO.new("abc").sync, StringIO.new("a").tty?, StringIO.new("a").fileno, StringIO.new("a").external_encoding
s = StringIO.new("abc")
s.string = "xyz"
p s.read, s.pos, s.reopen("new").class, s.read
s = StringIO.new("héllo")
p s.getc, s.getc, s.pos, s.getbyte, s.read, s.size
s = StringIO.new("abc")
s.ungetc("xy")
p s.read, s.string
s = StringIO.new("abc")
s.read
s.ungetbyte(65)
p s.read, s.pos
s = StringIO.new("a,b,c")
p s.gets(","), s.gets(",", chomp: true), s.gets(","), s.gets(",")
s = StringIO.new("ab\ncd\n")
p s.each_line(1).to_a, s.rewind, s.readlines(3), StringIO.new("abc").pread(2, 1), StringIO.open("q") { |q| q.read }
s = StringIO.new("abc")
p s.readpartial(2), s.read_nonblock(5), StringIO::VERSION, StringIO.ancestors.include?(Enumerable)
s = StringIO.new("abc")
s.close
p s.closed?
s = StringIO.new("abc")
s.close_read
p s.closed?, s.closed_read?, s.closed_write?, s.write("z"), s.string
[-> { StringIO.new("abc", "a").read }, -> { StringIO.new("abc", "r").write("d") }, -> { StringIO.new("abc", "w").read }, -> { x = StringIO.new("abc"); x.close; x.read }, -> { x = StringIO.new("abc"); x.read; x.readline }, -> { x = StringIO.new("abc"); x.read; x.readchar }, -> { x = StringIO.new("abc"); x.read; x.readpartial(1) }, -> { StringIO.new("abc").read(-1) }, -> { StringIO.new("abc").seek(-1) }, -> { StringIO.new("abc").seek(0, 9) }, -> { StringIO.new(1) }, -> { StringIO.new("abc", "x") }].each do |f|
  f.call
rescue StandardError => e
  puts "#{e.class}: #{e.message}"
end
`)
	assert.Equals(t, out, `"lin"
3
"e1\nline2\nlast"
""
nil
true
16
16
"line1\nline2\nlast"
0
"line1\n"
1
"line2"
"last"
nil
true
["line1\n", "line2\n", "last"]
0
["line1", "line2", "last"]
0
["line1\n", "line2\n", "last"]
0
["LINE1", "LINE2", "LAST"]
0
[6, 6, 4]
"l"
"i"
"n"
101
4
nil
3
"Z1\n"
["l", "i"]
8
"linZ1\nline2\nlast"
"linZ1\nline2"
"\n"
"last"
nil
0
""
"linZ1\nline2\nlast"
nil
""
""
"line2"
0
"linZ1"
0
"last"
0
0
"Z1"
5
0
[108, 105, 110]
["a\n\n", "b\n\n\n", "c"]
["a", "b", "c"]
[]
nil
""
nil
nil
5
7
"hello world1"
12
"hello world1!?"
nil
nil
nil
"hello world1!?a\nb\nc\nx1007|s"
"hello there"
11
2
"hello there!!"
0
1
"Jello there!!"
"ello there!!"
0
"he"
0
"he\u0000\u0000"
4
nil
"heX\no"
4
"abcd"
4
""
1
"x"
"\n\na\nb"
[97, 98, 0, 0, 0, 120]
11
true
false
nil
#<Encoding:UTF-8>
"xyz"
3
StringIO
"new"
"h"
"é"
3
108
"lo"
6
"xyabc"
"xyabc"
"A"
3
"a,"
"b"
"c"
nil
["a", "b", "\n", "c", "d", "\n"]
0
["ab\n", "cd\n"]
"bc"
"q"
"ab"
"c"
"3.1.2"
true
true
false
true
false
1
"zbc"
IOError: not opened for reading
IOError: not opened for writing
IOError: not opened for reading
IOError: not opened for reading
EOFError: end of file reached
EOFError: end of file reached
EOFError: end of file reached
ArgumentError: negative length -1 given
Errno::EINVAL: Invalid argument
Errno::EINVAL: Invalid argument - invalid whence
TypeError: no implicit conversion of Integer into String
ArgumentError: invalid access mode x
`)
}
