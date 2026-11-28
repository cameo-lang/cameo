# csv: testcase to demonstrate that CSV rows and tables are Ruby's as values
#
# The expected strings are Ruby 3.4's, with csv 3.3.2.
#
# (Introduced in PR #270. expected output follows the __END__ separator.)
require "csv"
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

__END__
#<CSV::Table mode:col_or_row row_count:3>
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
