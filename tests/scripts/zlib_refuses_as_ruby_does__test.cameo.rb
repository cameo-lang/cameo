# zlib: testcase to demonstrate that zlib refuses as Ruby does
#
# gunzip's errors are Ruby's for a member cut short at each point, an
# unsupported method or flag; a stream with a dictionary is NeedDict; gzip
# refuses a stray keyword and accepts strategy:; a continued adler32 equals
# the sum over the whole; the streaming constructors say what is left out. The
# dictionary stream is Ruby 3.4.1's.
#
# The expected values are Ruby's; the compressed bytes are compared by what
# they inflate to, since Go's deflate and zlib's differ.
#
# (Introduced in PR #275. expected output follows the __END__ separator.)
require "zlib"
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
begin; Zlib::Inflate.inflate(b"\x78\xbb\x02\x4d\x01\x27\x4b\x04\x23\x00\x08\x0c\x02\x4d"); rescue Zlib::NeedDict => e; puts "#{e.class}: #{e.message}"; end
p Zlib.adler32("b" * 7000, Zlib.adler32("a" * 20000)) == Zlib.adler32("a" * 20000 + "b" * 7000)

__END__
Zlib::GzipFile::Error: unexpected end of string
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
Zlib::Deflate.new is not supported: a stream fed piece by piece; use Zlib::Deflate.deflate on the whole string (see https://15b.it/lib)
Zlib::NeedDict: need dictionary
true
