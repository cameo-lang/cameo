# csv: testcase to demonstrate that CSV parses as Ruby does
#
# The expected strings are Ruby 3.4's, with csv 3.3.2.
#
# (Introduced in PR #270. expected output follows the __END__ separator.)
require "csv"
p CSV.parse("a,b\r\n1,2\r\n"), CSV.parse("a,\"b,c\",\"d\"\"e\"\n"), CSV.parse("a,\"multi\nline\",c\n"), CSV.parse(""), CSV.parse("\n\n"), CSV.parse("a,,b\n,,\n"), CSV.parse("a,\"\",b\n")
p CSV.parse("a;b\n1;2", col_sep: ";"), CSV.parse("a,b\n\n1,2\n", skip_blanks: true), CSV.parse("a,b\n\n1,2\n")
p CSV.parse("1,2.5,x,,1e3,0x10, 7 \n", converters: :numeric), CSV.parse("1,2.5,x\n", converters: :integer), CSV.parse("1,2.5,x\n", converters: :float), CSV.parse("1\n", converters: [:numeric, :numeric])
p CSV.parse("1,2.5,2020-01-02,2020-01-02T03:04:05Z,x\n", converters: :all), CSV.parse("2020-01-02,x,2020-13-45\n", converters: :date)
p CSV.parse("1,2\n", converters: [:integer, ->(f) { f.is_a?(String) ? f.upcase : f }]), CSV.parse("a,b\n", converters: ->(f, info) { "#{f}@#{info.index}:#{info.line}" })
p CSV.parse(" a , b \n", strip: true), CSV.parse("a,,\"\"\n", nil_value: "N", empty_value: "E"), CSV.parse("#c\na,b\n", skip_lines: /^#/), CSV.parse("a,b\"c\n", liberal_parsing: true)
p CSV.parse_line("a,\"b\"\"c\",,d\n"), CSV.parse_line(""), "a,b\n".parse_csv, "x;y".parse_csv(col_sep: ";")
CSV.parse("a,b\n1,2\n") { |row| p row }
["a,b\"c\n", "\"abc\n", "a,\"b\"c\n", " \"a\",b\n", "a\n\"b\",c\"\n", "a,b\n\"1\",\"2\"\"\n"].each do |text|
  CSV.parse(text)
rescue CSV::MalformedCSVError => e
  p [e.message, e.line_number]
end
begin; CSV.parse("x", col_sep: ""); rescue ArgumentError => e; p e.message; end
begin; CSV.parse(nil); rescue ArgumentError => e; p e.message; end
begin; CSV.parse("a", nope: 1); rescue ArgumentError => e; p e.message; end
p CSV::VERSION, CSV::MalformedCSVError.ancestors.take(3)

__END__
[["a", "b"], ["1", "2"]]
[["a", "b,c", "d\"e"]]
[["a", "multi\nline", "c"]]
[]
[[], []]
[["a", nil, "b"], [nil, nil, nil]]
[["a", "", "b"]]
[["a", "b"], ["1", "2"]]
[["a", "b"], ["1", "2"]]
[["a", "b"], [], ["1", "2"]]
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
["Illegal quoting in line 2.", 2]
["Unclosed quoted field in line 2.", 2]
":col_sep must be 1 or more characters: \"\""
"Cannot parse nil as CSV"
"unknown keyword: :nope"
"3.3.2"
[CSV::MalformedCSVError, RuntimeError, StandardError]
