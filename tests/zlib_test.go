package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby's; the compressed bytes are compared by
// what they inflate to, since Go's deflate and zlib's differ.

func TestZlibChecksumsStreamsAndGzipFollowRubys(t *testing.T) {
	out := runScript(t, `require "zlib"
s = "hello hello hello hello world"
p Zlib.crc32("hello"), Zlib.crc32(""), Zlib.crc32, Zlib.crc32("world", Zlib.crc32("hello ")), Zlib.crc32("hello world"), Zlib.crc32("\xff\x00".b)
p Zlib.adler32("hello"), Zlib.adler32(""), Zlib.adler32, Zlib.adler32("world", Zlib.adler32("hello ")), Zlib.crc_table.size, Zlib.crc_table[1], Zlib.crc_table[255]
d = Zlib::Deflate.deflate(s)
p d.encoding, d.bytes.first(2), Zlib::Inflate.inflate(d), Zlib::Inflate.inflate(d).encoding, Zlib.inflate(Zlib.deflate(s)) == s, Zlib::Inflate.inflate(d + "x")
p Zlib::Deflate.deflate(s, Zlib::BEST_COMPRESSION).bytes.first(2), Zlib::Deflate.deflate(s, Zlib::NO_COMPRESSION).bytes.first(2), Zlib::Deflate.deflate(s, 1).bytes.first(2), Zlib::Inflate.inflate(Zlib::Deflate.deflate("")), Zlib::Deflate.deflate("x" * 1000).bytesize < 50
p Zlib::Inflate.inflate(["789ccb48cdc9c95728cf2fca4901001a0b045d"].pack("H*")), Zlib::Inflate.inflate(Zlib::Deflate.deflate("日本語")).force_encoding("UTF-8")
g = Zlib.gzip(s)
p g.bytes.first(4), g.bytes[8, 2], g.encoding, Zlib.gunzip(g), g.bytes.last(8), Zlib.gzip(s, level: 9).bytes[8], Zlib.gzip(s, level: 1).bytes[8], Zlib.gunzip(Zlib.gzip("")), Zlib.gzip("x" * 1000).bytesize < 60, Zlib.gunzip(g + "trailing")
p Zlib.gunzip(["1f8b0800dc37e96a0003cb48cdc9c957c8c020cbf38b725200336534341d000000"].pack("H*"))
["garbage", "", "\x78\x9c".b, d[0, 10], d[0..-2] + "\x00"].each do |bad|
  Zlib::Inflate.inflate(bad)
rescue Zlib::Error => e
  puts "#{e.class}: #{e.message}"
end
[g[0, 12], "garbage", d, g[0..-9], ["1f8b0800000000000003cb48cdc9c957284f2fca4901002d3b08af0b000000"].pack("H*")].each do |bad|
  Zlib.gunzip(bad)
rescue Zlib::Error => e
  puts "#{e.class}: #{e.message}"
end
begin; Zlib::Deflate.deflate(s, 10); rescue Zlib::Error => e; puts "#{e.class}: #{e.message}"; end
begin; Zlib.gzip(s, level: 10); rescue Zlib::Error => e; puts "#{e.class}: #{e.message}"; end
begin; Zlib::Deflate.deflate(nil); rescue TypeError => e; puts e.message; end
p [Zlib::NO_COMPRESSION, Zlib::BEST_SPEED, Zlib::BEST_COMPRESSION, Zlib::DEFAULT_COMPRESSION, Zlib::DEFAULT_STRATEGY, Zlib::FINISH, Zlib::MAX_WBITS]
p [Zlib::Error.superclass, Zlib::DataError.superclass, Zlib::BufError.superclass, Zlib::GzipFile::Error.superclass, Zlib::GzipFile::CRCError.superclass, Zlib::VERSION, Zlib::ZLIB_VERSION, Zlib.zlib_version]
`)
	assert.Equals(t, out, `907060870
0
0
222957957
222957957
3539857293
103547413
1
1
436929629
256
1996959894
755167117
#<Encoding:BINARY (ASCII-8BIT)>
[120, 156]
"hello hello hello hello world"
#<Encoding:BINARY (ASCII-8BIT)>
true
"hello hello hello hello world"
[120, 218]
[120, 1]
[120, 1]
""
true
"hello world"
"日本語"
[31, 139, 8, 0]
[0, 3]
#<Encoding:BINARY (ASCII-8BIT)>
"hello hello hello hello world"
[51, 101, 52, 52, 29, 0, 0, 0]
2
4
""
true
"hello hello hello hello world"
"hello hello hello hello world"
Zlib::DataError: incorrect header check
Zlib::BufError: buffer error
Zlib::BufError: buffer error
Zlib::BufError: buffer error
Zlib::DataError: incorrect data check
Zlib::GzipFile::Error: unexpected end of file
Zlib::GzipFile::Error: unexpected end of string
Zlib::GzipFile::Error: not in gzip format
Zlib::GzipFile::NoFooter: footer is not found
Zlib::GzipFile::CRCError: invalid compressed data -- crc error
Zlib::StreamError: stream error
Zlib::StreamError: stream error
no implicit conversion of nil into String
[0, 1, 9, -1, 0, 4, 15]
[StandardError, Zlib::Error, Zlib::Error, Zlib::Error, Zlib::GzipFile::Error, "3.2.1", "1.3.1", "1.3.1"]
`)
}

// gunzip's errors are Ruby's for a member cut short at each point, an
// unsupported method or flag; a stream with a dictionary is NeedDict;
// gzip refuses a stray keyword and accepts strategy:; a continued
// adler32 equals the sum over the whole; the streaming constructors say
// what is left out. The dictionary stream is Ruby 3.4.1's.
func TestZlibRefusesAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "zlib"
g = Zlib.gzip("hello")
[g[0..-5], g[0..-2], g[0..-9], g[0..-10], g[0..9], g[0..3]].each do |s|
  Zlib.gunzip(s)
rescue Zlib::Error => e
  puts "#{e.class}: #{e.message}"
end
begin; Zlib.gunzip(g.b[0, 2] + "\x09".b + g.b[3..]); rescue Zlib::Error => e; puts "#{e.class}: #{e.message}"; end
[0x02, 0x20, 0x40, 0x80].each { |f| flagged = g.b[0, 3] + f.chr.b + g.b[4..]; begin; Zlib.gunzip(flagged); rescue Zlib::Error => e; puts e.message; end }
begin; Zlib.gunzip(g.b[0, 3] + "\x04".b + g.b[4, 6] + "\x05\x00".b); rescue Zlib::Error => e; puts e.message; end
begin; Zlib.gzip("x", levl: 9); rescue ArgumentError => e; puts e.message; end
p Zlib.gzip("x", strategy: Zlib::HUFFMAN_ONLY).bytesize, Zlib.gzip("x", level: 9).getbyte(8), Zlib.gzip("x", level: 1).getbyte(8)
begin; Zlib::Deflate.new(9); rescue NotImplementedError => e; puts e.message; end
begin; Zlib::Inflate.inflate("\x78\xbb\x02\x4d\x01\x27\x4b\x04\x23\x00\x08\x0c\x02\x4d".b); rescue Zlib::NeedDict => e; puts "#{e.class}: #{e.message}"; end
p Zlib.adler32("b" * 7000, Zlib.adler32("a" * 20000)) == Zlib.adler32("a" * 20000 + "b" * 7000)
`)
	assert.Equals(t, out, `Zlib::GzipFile::Error: unexpected end of string
Zlib::GzipFile::Error: unexpected end of string
Zlib::GzipFile::NoFooter: footer is not found
Zlib::GzipFile::Error: unexpected end of file
Zlib::GzipFile::Error: unexpected end of file
Zlib::GzipFile::Error: unexpected end of string
Zlib::GzipFile::Error: unsupported compression method 9
multi-part gzip file is not supported
encrypted gzip file is not supported
unknown flags 0x40
unknown flags 0xffffff80
unexpected end of string
unknown keyword: :levl
25
2
4
Zlib::Deflate.new is not supported: a stream fed piece by piece; use Zlib::Deflate.deflate on the whole string
Zlib::NeedDict: need dictionary
true
`)
}
