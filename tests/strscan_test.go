package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby 3.4's, with strscan 3.1.2.

func TestStringScannerScansAsRubysDoes(t *testing.T) {
	out := runScript(t, `require "strscan"
s = StringScanner.new("3.14 is pi, 42 the answer")
p s.scan(/\d+/), s.scan(/\./), s.scan(/\d+/), s.matched, s.matched?, s.matched_size, s.pre_match, s.post_match, s.pos, s.charpos, s.eos?, s.rest, s.rest_size, s.string
p s.scan(/\d+/), s.matched, s.matched?, s.pre_match, s.skip(/\s+/), s.check(/\w+/), s.pos, s.scan_until(/,/), s.matched, s.pre_match, s.pos, s.check_until(/\d+/), s.pos, s.skip_until(/\d+/), s.pos, s.match?(/\s/), s.pos, s.peek(4), s.getch, s.rest
s.reset
p s.scan(/(\d+)\.(\d+)/), s[0], s[1], s[2], s[3], s[-1], s.captures, s.values_at(0, 2), s.size, s.unscan.pos, s.scan(/(?<int>\d+)\.(?<frac>\d+)/), s[:int], s["frac"], s.named_captures, s.pos, s[9]
s.terminate
p s.eos?, s.pos, s.scan(/x/), s.matched, s.rest, s.getch, s.peek(2), s.scan_until(/x/), s.check(/x/)
s.reset
p s.scan("3.14"), s.scan(" is"), s.scan("x"), s.pos, s.scan_until(","), s.skip("42"), s.skip(" 42"), s.pos, s.exist?(/answer/), s.pos, s.exist?(/zzz/), s.search_full(/the/, false, false), s.pos, s.search_full(/the/, true, true), s.pos, s.scan_full(/ /, false, true), s.scan_full(/an/, true, false), s.pos
s.reset
p s.scan(/^\d/), s.scan(/^\./), s.scan(/\A\d+/), s.scan(/\s/), s.scan(/^is/), s.pos, s.beginning_of_line?, s.reset.inspect, s.bol?
s.reset
p s.get_byte, s.pos, s.getch, s.pos, s.reset.scan_byte, s.peek_byte, s.pos
x = StringScanner.new("héllo")
p x.getch, x.getch, x.pos, x.charpos, x.get_byte, x.pos, x.rest, x.peek(1), x.scan(/l+/), x.matched_size
p StringScanner.new("").inspect, StringScanner.new("a" * 20).inspect, StringScanner.new("a" * 20).tap { |z| z.pos = 10 }.inspect
x = StringScanner.new("abc")
x.pos = 1
p x.pos, x.scan(/b/)
x.pos = -1
p x.pos, x.scan(/c/)
x = StringScanner.new("ab")
x.string = "xyz"
p x.pos, x.scan(/x/), x.string, x.concat("w").string, (x << "v").rest, x.scan(/yzwv/)
p StringScanner::Version, StringScanner::Error.superclass, StringScanner.new("a").fixed_anchor?, StringScanner.new("a", fixed_anchor: true).fixed_anchor?
x = StringScanner.new("abc", fixed_anchor: true)
p x.scan(/\Aa/), x.scan(/\Ab/), x.scan(/b/), x.scan(/^c/), x.scan(/c/)
x = StringScanner.new("foo bar")
p x.scan(/(f)(o)(o)/), x[0], x[1], x[3], x.pre_match, x.post_match, x.scan(/\s/), x.pre_match, x.post_match, x.matched
x = StringScanner.new("foo")
p x.check(/f/), x.pos, x.matched, x.match?(/fo/), x.matched, x.pos, x.scan(/z/), x.matched, x[0], x.pre_match
x = StringScanner.new("a\nb")
p x.scan(/a/), x.bol?, x.scan(/\n/), x.bol?, x.scan(/^b/), x.eos?
x = StringScanner.new("abc")
p x.scan(/(?<x>a)|(?<y>b)/), x.named_captures, x.scan(/(?<x>a)|(?<y>b)/), x.named_captures, x[:x], x[:y], x.captures, x.rest?, x.scan(/c/), x.rest?, x.empty?
x = StringScanner.new("abc")
p x.peek(0), x.peek(10), x.getch, x.getch, x.getch, x.getch, x.get_byte, x.scan_byte, x.peek_byte
[-> { StringScanner.new("abc").unscan }, -> { y = StringScanner.new("abc"); y.scan(/x/); y.unscan }, -> { y = StringScanner.new("abc"); y.scan(/a/); y[:nope] }, -> { y = StringScanner.new("abc"); y.pos = 10 }, -> { StringScanner.new("abc").scan(1) }, -> { StringScanner.new("abc").peek(-1) }].each do |f|
  f.call
rescue StandardError => e
  puts "#{e.class}: #{e.message}"
end
`)
	assert.Equals(t, out, `"3"
"."
"14"
"14"
true
2
"3."
" is pi, 42 the answer"
4
4
false
" is pi, 42 the answer"
21
"3.14 is pi, 42 the answer"
nil
nil
false
nil
1
"is"
5
"is pi,"
","
"3.14 is pi"
11
" 42"
11
3
14
1
14
" the"
" "
"the answer"
"3.14"
"3.14"
"3"
"14"
nil
"14"
["3", "14"]
["3.14", "14"]
3
0
"3.14"
"3"
"14"
{"int" => "3", "frac" => "14"}
4
nil
true
25
nil
nil
""
nil
""
nil
nil
"3.14"
" is"
nil
7
" pi,"
nil
3
14
11
14
nil
4
14
" the"
18
" "
nil
18
"3"
"."
"14"
" "
"is"
7
false
"#<StringScanner 0/25 @ \"3.14 ...\">"
true
"3"
1
"."
2
51
46
1
"h"
"é"
3
2
"l"
4
"lo"
"l"
"l"
1
"#<StringScanner fin>"
"#<StringScanner 0/20 @ \"aaaaa...\">"
"#<StringScanner 10/20 \"...aaaaa\" @ \"aaaaa...\">"
1
"b"
2
"c"
0
"x"
"xyz"
"xyzw"
"yzwv"
"yzwv"
"3.1.2"
StandardError
false
true
"a"
nil
"b"
nil
"c"
"foo"
"foo"
"f"
"o"
""
" bar"
" "
"foo"
"bar"
" "
"f"
0
"f"
2
"fo"
0
nil
nil
nil
nil
"a"
false
"\n"
true
"b"
true
"a"
{"x" => "a", "y" => nil}
"b"
{"x" => nil, "y" => "b"}
nil
"b"
[nil, "b"]
true
"c"
false
true
""
"abc"
"a"
"b"
"c"
nil
nil
nil
nil
StringScanner::Error: unscan failed: previous match record not exist
StringScanner::Error: unscan failed: previous match record not exist
IndexError: undefined group name reference: nope
RangeError: index out of range
TypeError: no implicit conversion of Integer into String
ArgumentError: negative string size (or size too big)
`)
}

// With fixed_anchor the leftmost match at or after the position is found
// from the position, with the anchors holding at the string's start;
// inspect quotes as Ruby's dump does; a stray keyword is refused;
// named_captures lists the last Regexp's names, nil when it did not
// match; a huge peek is clamped; a subclass's instances are the
// subclass's. The answers are Ruby 3.4.1's.
func TestStringScannerFixedAnchorsAndRubysForms(t *testing.T) {
	out := runScript(t, `require "strscan"
p StringScanner.new("a\nb\t\"\\").inspect, StringScanner.new("héllo").inspect
s = StringScanner.new("héllo"); s.pos = 2; p s.inspect
s = StringScanner.new("aab", fixed_anchor: true); s.pos = 1; p s.scan(/a+/), s.scan(/\Ab/), s.scan(/^b/), s.scan(/b\z/)
s = StringScanner.new("123 456", fixed_anchor: true); s.pos = 1; p s.scan(/\d+/), s.pos, s.scan_until(/\d+/), s.pos
s = StringScanner.new("x\nab", fixed_anchor: true); s.pos = 2; p s.scan(/^a/), s.scan(/\Ab/)
begin; StringScanner.new("a", fixed_anchors: true); rescue ArgumentError => e; puts e.message; end
z = StringScanner.new("abc"); z.scan(/(?<x>a)/); z.scan(/(?<y>z)/); p z.named_captures; z.scan("b"); p z.named_captures
s = StringScanner.new("abc"); s.pos = 1; p s.peek(2**63 - 1)
class Lexer < StringScanner
  def hello
    "hi"
  end
end
p Lexer.new("q").class, Lexer.new("q").hello
`)
	assert.Equals(t, out, `"#<StringScanner 0/6 @ \"a\\nb\\t\\\"...\">"
"#<StringScanner 0/6 @ \"h\\xC3\\xA9ll...\">"
"#<StringScanner 2/6 \"h\\xC3\" @ \"\\xA9llo\">"
"a"
nil
nil
"b"
"23"
3
" 456"
7
"a"
nil
unknown keyword: :fixed_anchors
{"y" => nil}
{"y" => nil}
"bc"
Lexer
"hi"
`)
}
