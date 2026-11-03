package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby 3.4's, with csv 3.3.2.

func TestCSVParsesAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "csv"
p CSV.parse("a,b\r\n1,2\r\n"), CSV.parse("a,\"b,c\",\"d\"\"e\"\n"), CSV.parse("a,\"multi\nline\",c\n"), CSV.parse(""), CSV.parse("\n\n"), CSV.parse("a,,b\n,,\n"), CSV.parse("a,\"\",b\n")
p CSV.parse("a;b\n1;2", col_sep: ";"), CSV.parse("a,b|1,2|", row_sep: "|"), CSV.parse("'a,b',c\n", quote_char: "'"), CSV.parse("a,b\n\n1,2\n", skip_blanks: true), CSV.parse("a,b\n\n1,2\n"), CSV.parse("a,b\r1,2\r")
p CSV.parse("1,2.5,x,,1e3,0x10, 7 \n", converters: :numeric), CSV.parse("1,2.5,x\n", converters: :integer), CSV.parse("1,2.5,x\n", converters: :float), CSV.parse("1\n", converters: [:numeric, :numeric])
p CSV.parse("1,2.5,2020-01-02,2020-01-02T03:04:05Z,x\n", converters: :all), CSV.parse("2020-01-02,x,2020-13-45\n", converters: :date)
p CSV.parse("1,2\n", converters: [:integer, ->(f) { f.is_a?(String) ? f.upcase : f }]), CSV.parse("a,b\n", converters: ->(f, info) { "#{f}@#{info.index}:#{info.line}" })
p CSV.parse(" a , b \n", strip: true), CSV.parse("a,,\"\"\n", nil_value: "N", empty_value: "E"), CSV.parse("#c\na,b\n", skip_lines: /^#/), CSV.parse("a,b\"c\n", liberal_parsing: true), CSV.parse("a,b\n", quote_char: nil)
p CSV.parse_line("a,\"b\"\"c\",,d\n"), CSV.parse_line(""), "a,b\n".parse_csv, "x;y".parse_csv(col_sep: ";")
CSV.parse("a,b\n1,2\n") { |row| p row }
["a,b\"c\n", "\"abc\n", "a,\"b\"c\n", " \"a\",b\n", "a\rb\n", "a\n\"b\",c\"\n", "a,b\n\"1\",\"2\"\"\n"].each do |text|
  CSV.parse(text)
rescue CSV::MalformedCSVError => e
  p [e.message, e.line_number]
end
begin; CSV.parse("x", col_sep: ""); rescue ArgumentError => e; p e.message; end
begin; CSV.parse(nil); rescue ArgumentError => e; p e.message; end
begin; CSV.parse("a", nope: 1); rescue ArgumentError => e; p e.message; end
p CSV::VERSION, CSV::MalformedCSVError.ancestors.take(3)
`)
	assert.Equals(t, out, `[["a", "b"], ["1", "2"]]
[["a", "b,c", "d\"e"]]
[["a", "multi\nline", "c"]]
[]
[[], []]
[["a", nil, "b"], [nil, nil, nil]]
[["a", "", "b"]]
[["a", "b"], ["1", "2"]]
[["a", "b"], ["1", "2"]]
[["a,b", "c"]]
[["a", "b"], ["1", "2"]]
[["a", "b"], [], ["1", "2"]]
[["a", "b"], ["1", "2"]]
[[1, 2.5, "x", nil, 1000.0, 16, 7]]
[[1, "2.5", "x"]]
[[1.0, 2.5, "x"]]
[[1]]
[[1, 2.5, #<DateTime: 2020-01-02T00:00:00+00:00 ((2458851j,0s,0n),+0s,2299161j)>, #<DateTime: 2020-01-02T03:04:05+00:00 ((2458851j,11045s,0n),+0s,2299161j)>, "x"]]
[[#<Date: 2020-01-02 ((2458851j,0s,0n),+0s,2299161j)>, "x", "2020-13-45"]]
[[1, 2]]
[["a@0:1", "b@1:1"]]
[["a", "b"]]
[["a", "N", "E"]]
[["a", "b"]]
[["a", "b\"c"]]
[["a", "b"]]
["a", "b\"c", nil, "d"]
nil
["a", "b"]
["x", "y"]
["a", "b"]
["1", "2"]
["Illegal quoting in line 1.", 1]
["Unclosed quoted field in line 1.", 1]
["Any value after quoted field isn't allowed in line 1.", 1]
["Illegal quoting in line 1.", 1]
["Unquoted fields do not allow new line <\"\\n\"> in line 2.", 2]
["Illegal quoting in line 2.", 2]
["Unclosed quoted field in line 2.", 2]
":col_sep must be 1 or more characters: \"\""
"Cannot parse nil as CSV"
"unknown keyword: :nope"
"3.3.2"
[CSV::MalformedCSVError, RuntimeError, StandardError]
`)
}

func TestCSVRowsAndTablesAreRubysAsValues(t *testing.T) {
	out := runScript(t, `require "csv"
tb = CSV.parse("a,b\n1,2\n3,4", headers: true)
p tb, tb.class, tb.to_a, tb.headers, tb.size, tb[0], tb["a"], tb[5], tb["z"], tb.by_col["a"], tb.by_col[0], tb.by_row[1], tb.mode, tb.by_col.mode, tb.to_s, tb.to_csv(col_sep: ";"), tb.to_csv(write_headers: false)
p tb.map { |r| r["b"] }, tb.by_col.map { |h, c| [h, c] }, tb.each.to_a.size, tb.values_at(0), tb.by_col.values_at("a"), tb.dig(0, "a"), tb.by_col.dig("a", 1), tb == CSV.parse("a,b\n1,2\n3,4", headers: true)
p (tb << [5, 6]).to_a, (tb << {"a" => 7, "b" => 8}).to_a, tb.delete("a").to_a, tb.delete(0).to_a, tb.delete_if { |r| r["a"] == "1" }.to_a, CSV::Table.new([], headers: ["x", "y"]).headers, CSV::Table.new([CSV::Row.new(["a"], [1])]).to_a
p CSV.parse("1,2\n3,4", headers: ["x", "y"]).to_a, CSV.parse("1,2", headers: "x,y").to_a, CSV.parse("a,b\n1,2", headers: true, return_headers: true).to_a, CSV.parse("a,b\n1,2", headers: true, return_headers: true).size
p CSV.parse("A b,C-d\n1,2", headers: true, header_converters: :symbol).first.to_h, CSV.parse("A b,C\n1,2", headers: true, header_converters: :downcase).headers, CSV.parse("a b,c\n", headers: true, header_converters: ->(h) { h.tr(" ", "_") }).headers
p CSV.parse("a,b\n1,2,3\n", headers: true).to_a, CSV.parse("a,b\n1\n", headers: true).first.to_h, CSV.parse("a,a\n1,2", headers: true).first.to_h, CSV.parse("a,b\n1,2", headers: true, converters: :numeric)["b"], CSV.parse("a,b", headers: true).size
r = CSV::Row.new(["a", "b"], [1, 2])
p r, r["a"], r[1], r[-1], r["z"], r.fetch("a"), r.fetch("z", 9), r.fetch("z") { |h| h * 2 }, r.to_h, r.to_a, r.headers, r.fields, r.size, r.header?("a"), r.field?(2), r.to_s, r.to_csv(col_sep: ";"), r.header_row?, r.dig("a"), r.index("b"), r.values_at("b", "a"), r.each.to_a, r.map { |h, v| "#{h}=#{v}" }, r.deconstruct_keys(nil), r.deconstruct
p r == CSV::Row.new(["a", "b"], [1, 2]), r == [1, 2], CSV::Row.new(["a"], [1], true).header_row?, CSV::Row.new(["a", "b"], [1]).to_a, CSV::Row.new(["a"], [1, 2]).to_a, CSV::Row.new(["a"], [1, 2]).headers, CSV::Row.new(["a", "a"], [1, 2])["a", 1], CSV::Row.new(["a", "b", "a"], [1, 2, 3]).to_h, CSV::Row.new([:a, "b"], [1, nil])
begin; r.fetch("z"); rescue KeyError => e; p e.message, e.key; end
r["a"] = 5
p r
r["c"] = 7
p r, r << 9, r << ["d", 4], r << {"e" => 5}, r.push(6).to_a, r.delete("a")
`)
	assert.Equals(t, out, `#<CSV::Table mode:col_or_row row_count:3>
a,b
1,2
3,4

CSV::Table
[["a", "b"], ["1", "2"], ["3", "4"]]
["a", "b"]
2
#<CSV::Row "a":"1" "b":"2">
["1", "3"]
nil
[nil, nil]
["1", "3"]
["1", "3"]
#<CSV::Row "a":"3" "b":"4">
:col_or_row
:col
"a,b\n1,2\n3,4\n"
"a;b\n1;2\n3;4\n"
"1,2\n3,4\n"
["2", "4"]
[["a", ["1", "3"]], ["b", ["2", "4"]]]
2
[#<CSV::Row "a":"1" "b":"2">]
[["1"], ["3"]]
"1"
"3"
true
[["a", "b"], ["1", "2"], ["3", "4"], [5, 6]]
[["a", "b"], ["1", "2"], ["3", "4"], [7, 8]]
[["b"], ["2"], ["4"]]
[["a", "b"], ["3", "4"]]
[["a", "b"], ["3", "4"]]
["x", "y"]
[["a"], [1]]
[["x", "y"], ["1", "2"], ["3", "4"]]
[["x", "y"], ["1", "2"]]
[["a", "b"], ["1", "2"]]
2
{a_b: "1", cd: "2"}
["a b", "c"]
["a_b", "c"]
[["a", "b", nil], ["1", "2", "3"]]
{"a" => "1", "b" => nil}
{"a" => "1"}
[2]
0
#<CSV::Row "a":1 "b":2>
1
2
2
nil
1
9
"zz"
{"a" => 1, "b" => 2}
[["a", 1], ["b", 2]]
["a", "b"]
[1, 2]
2
true
true
"1,2\n"
"1;2\n"
false
1
1
[2, 1]
[["a", 1], ["b", 2]]
["a=1", "b=2"]
{"a" => 1, "b" => 2}
[1, 2]
true
false
true
[["a", 1], ["b", nil]]
[["a", 1], [nil, 2]]
["a", nil]
2
{"a" => 1, "b" => 2}
#<CSV::Row a:1 "b":nil>
"key not found: z"
"z"
#<CSV::Row "a":5 "b":2>
#<CSV::Row "a":5 "b":2 "c":7>
#<CSV::Row "a":5 "b":2 "c":7 nil:9>
#<CSV::Row "a":5 "b":2 "c":7 "d":4>
#<CSV::Row "a":5 "b":2 "c":7 "e":5>
[["a", 5], ["b", 2], ["c", 7], [nil, 6]]
#<CSV::Row "b":2 "c":7>
`)
}

func TestCSVGeneratesAndReadsAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "csv"
p CSV.generate_line(["a", nil, "", 1, 2.5, :sym, true, "x,y", "q\"r", "l\nm", " s ", "t\rr"]), CSV.generate_line(["a", "b"], col_sep: ";", row_sep: "|"), CSV.generate_line(["a", "b"], force_quotes: true), CSV.generate_line(["a", "", nil], quote_empty: false), CSV.generate_line(["a,b"], quote_char: "'"), CSV.generate_line([]), CSV.generate_line([nil]), CSV.generate_line([[1, 2], {a: 1}])
p CSV.generate { |csv| csv << ["a", "b"]; csv << [1, nil] }, CSV.generate("x,y\n") { |csv| csv << [1, 2] }, CSV.generate(col_sep: "\t") { |csv| csv << ["a", "b"] }, CSV.generate(headers: ["a", "b"], write_headers: true) { |csv| csv << [1, 2]; csv << {"b" => 4, "a" => 3} }, CSV.generate(headers: ["a", "b"]) { |csv| csv << {"a" => 1} }, CSV.generate(headers: true) { |csv| csv << ["a", "b"]; csv << {"b" => 2, "a" => 1} }
p CSV.generate_lines([["a", "b"], [1, 2]]), ["a", "b,c"].to_csv, ["a"].to_csv(col_sep: ";"), CSV.generate { |csv| csv << CSV::Row.new(["a", "b"], [1, 2]) }, CSV.generate(force_quotes: true) { |csv| csv << [1, nil, ""] }, CSV.generate_line([nil, ""], write_nil_value: "N", write_empty_value: "E"), CSV.generate_line(["a", "b"], row_sep: "\r\n"), CSV.generate_line([Time.utc(2020, 1, 2, 3, 4, 5), Date.new(2020, 1, 2), 1..2, [1]]), CSV.generate_line(["a", "b"], headers: ["x", "y"], write_headers: true)
c = CSV.new("a,b\n1,2\n")
p c.lineno, c.line, c.headers, c.col_sep, c.row_sep, c.quote_char, c.eof?, c
p c.shift, c.lineno, c.line, c.shift, c.eof?, c.shift
p CSV.new("a,b\n1,2\n").read, CSV.new("a,b\n1,2\n", headers: true).read.class, CSV.new("a,b\n1,2\n").readlines, CSV.new("a,b\n1,2\n").each.to_a, CSV.new("a,b\n1,2\n").to_a, CSV.new("a,b\n1,2\n").map(&:first), CSV.instance("x,y\n").read, CSV.new("a,b\n1,2\n").string
h = CSV.new("a,b\n1,2\n", headers: true)
p h.headers, h.shift, h.headers
w = CSV.new("")
w << [1, 2]
w << ["x", nil]
p w.string
`)
	assert.Equals(t, out, `"a,,\"\",1,2.5,sym,true,\"x,y\",\"q\"\"r\",\"l\nm\", s ,\"t\rr\"\n"
"a;b|"
"\"a\",\"b\"\n"
"a,,\n"
"'a,b'\n"
"\n"
"\n"
"\"[1, 2]\",{a: 1}\n"
"a,b\n1,\n"
"x,y\n1,2\n"
"a\tb\n"
"a,b\n1,2\n3,4\n"
"1,\n"
"a,b\n1,2\n"
"a,b\n1,2\n"
"a,\"b,c\"\n"
"a\n"
"1,2\n"
"\"1\",\"\",\"\"\n"
"N,E\n"
"a,b\r\n"
"2020-01-02 03:04:05 UTC,2020-01-02,1..2,[1]\n"
"x,y\na,b\n"
0
nil
nil
","
"\n"
"\""
false
#<CSV io_type:StringIO encoding:UTF-8 lineno:0 col_sep:"," row_sep:"\n" quote_char:"\"">
["a", "b"]
1
"a,b\n"
["1", "2"]
true
nil
[["a", "b"], ["1", "2"]]
CSV::Table
[["a", "b"], ["1", "2"]]
[["a", "b"], ["1", "2"]]
[["a", "b"], ["1", "2"]]
["a", "1"]
[["x", "y"]]
"a,b\n1,2\n"
true
#<CSV::Row "a":"1" "b":"2">
["a", "b"]
"1,2\nx,\n"
`)
}
