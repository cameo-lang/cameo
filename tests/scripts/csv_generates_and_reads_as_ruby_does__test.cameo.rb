# csv: testcase to demonstrate that CSV generates and reads as Ruby does
#
# The expected strings are Ruby 3.4's, with csv 3.3.2.
#
# (Introduced in PR #270. expected output follows the __END__ separator.)
require "csv"
p CSV.generate_line(["a", nil, "", 1, 2.5, :sym, true, "x,y", "q\"r", "l\nm", " s ", "t\rr"]), CSV.generate_line(["a", "b"], col_sep: ";"), CSV.generate_line([]), CSV.generate_line([nil]), CSV.generate_line([[1, 2], {a: 1}])
p CSV.generate { |csv| csv << ["a", "b"]; csv << [1, nil] }, CSV.generate("x,y\n") { |csv| csv << [1, 2] }, CSV.generate(col_sep: "\t") { |csv| csv << ["a", "b"] }, CSV.generate(headers: ["a", "b"], write_headers: true) { |csv| csv << [1, 2]; csv << {"b" => 4, "a" => 3} }, CSV.generate(headers: ["a", "b"]) { |csv| csv << {"a" => 1} }, CSV.generate(headers: true) { |csv| csv << ["a", "b"]; csv << {"b" => 2, "a" => 1} }
p CSV.generate_lines([["a", "b"], [1, 2]]), ["a", "b,c"].to_csv, ["a"].to_csv(col_sep: ";"), CSV.generate { |csv| csv << CSV::Row.new(["a", "b"], [1, 2]) }, CSV.generate_line([nil, ""], write_nil_value: "N", write_empty_value: "E"), CSV.generate_line(["a", "b"], row_sep: "\r\n"), CSV.generate_line([Time.utc(2020, 1, 2, 3, 4, 5), Date.new(2020, 1, 2), 1..2, [1]]), CSV.generate_line(["a", "b"], headers: ["x", "y"], write_headers: true)
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

__END__
"a,,,1,2.5,sym,true,\"x,y\",\"q\"\"r\",\"l\nm\",\" s \",\"t\rr\"\n"
"a;b\n"
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
