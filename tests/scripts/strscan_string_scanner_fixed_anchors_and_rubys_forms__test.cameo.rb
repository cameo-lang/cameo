# strscan: testcase to demonstrate that string scanner fixed anchors and
# Ruby's forms
#
# With fixed_anchor the leftmost match at or after the position is found from
# the position, with the anchors holding at the string's start; inspect quotes
# as Ruby's dump does; a stray keyword is refused; named_captures lists the
# last Regexp's names, nil when it did not match; a huge peek is clamped; a
# subclass's instances are the subclass's. The answers are Ruby 3.4.1's.
#
# The expected values are Ruby 3.4's, with strscan 3.1.2.
#
# (Introduced in PR #287. expected output follows the __END__ separator.)
require "strscan"
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

__END__
"#<StringScanner 0/6 @ \"a\\nb\\t\\\"...\">"
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
