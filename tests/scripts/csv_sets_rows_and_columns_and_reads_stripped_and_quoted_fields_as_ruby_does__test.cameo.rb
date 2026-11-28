# csv: testcase to demonstrate that CSV sets rows and columns and reads
# stripped and quoted fields as Ruby does
#
# Table#[]= sets a row or a column, Row#[]= counts a negative index from the
# end, strip: takes its characters away from around a quoted field too,
# FieldInfo#quoted? says whether the field was, a malformed line is numbered
# as Ruby numbers it with the empty lines before it, headers: true writes the
# first row once, and write_nil_value: and write_empty_value: take any value.
# Each line is what Ruby 3.4.1 prints.
#
# The expected strings are Ruby 3.4's, with csv 3.3.2.
#
# (Introduced in PR #270. expected output follows the __END__ separator.)
require "csv"
tb = CSV.parse("a,b\n1,2\n3,4", headers: true)
tb[0]["a"] = 9; p tb.to_a
tb["b"] = [7, 8]; p tb.to_a
tb["c"] = 5; p tb.to_a
tb[1] = CSV::Row.new(%w[a b c], [0, 0, 0]); tb[2] = [1, 1, 1]; p tb.to_a
t2 = tb.by_col; t2[0] = [5, 5, 5]; p t2.to_a
p CSV.parse("a\n1\n2\n3\n4\n5\n6\n7", headers: true).inspect, CSV.parse("a\n1\n2", headers: true).to_csv(limit: 1)
r = CSV::Row.new(%w[a b], [1, 2]); r[-1] = 5; p r
begin; r[-5] = 5; rescue IndexError => e; puts e.message; end
p CSV.parse_line("*a*,*b*", strip: "*"), CSV.parse_line(" \"a\" , b ", strip: true), CSV.parse_line("*\"a\"*,*b*", strip: "*"), CSV.parse_line(" a , b ", strip: "*")
p CSV.parse("a,b\n\"x\" ,\" y\"\n", strip: true), CSV.parse_line("\"1\",2", converters: ->(f, i) { i.quoted? })
begin; CSV.parse("a\n\n\"b"); rescue CSV::MalformedCSVError => e; puts "#{e.message} #{e.line_number}"; end
begin; CSV.parse("\"a\nb\"\n\"c"); rescue CSV::MalformedCSVError => e; puts e.message; end
begin; CSV.parse("a\n\nb\"c"); rescue CSV::MalformedCSVError => e; puts e.message; end
p CSV.generate(headers: true, write_headers: true) { |c| c << ["a", "b"]; c << [1, 2] }
p CSV.generate_line([""], write_empty_value: 0), CSV.generate_line([nil], write_nil_value: 0)

__END__
[["a", "b"], [9, "2"], ["3", "4"]]
[["a", "b"], [9, 7], ["3", 8]]
[["a", "b", "c"], [9, 7, 5], ["3", 8, 5]]
[["a", "b", "c"], [9, 7, 5], [0, 0, 0], [1, 1, 1]]
[["a", "b", "c"], [5, 7, 5], [5, 0, 0], [5, 1, 1]]
"#<CSV::Table mode:col_or_row row_count:8>\na\n1\n2\n3\n4\n5\n"
"a\n1\n"
#<CSV::Row "a":1 "b":5>
index -5 too small for array; minimum: -2
["a", "b"]
["a", "b"]
["a", "b"]
[" a ", " b "]
[["a", "b"], ["x", " y"]]
[true, false]
Unclosed quoted field in line 3. 3
Unclosed quoted field in line 2.
Illegal quoting in line 3.
"a,b\n1,2\n"
"0\n"
"0\n"
