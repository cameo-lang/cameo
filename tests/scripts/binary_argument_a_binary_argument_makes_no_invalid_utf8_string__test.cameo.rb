# binary argument: testcase to demonstrate that a binary argument makes no
# invalid UTF-8 string
#
# A method of a UTF-8 string does not put the bytes of a binary argument into
# a string of characters: it raises, and what joins strings makes a binary
# string. No string a script gets is UTF-8 and invalid.
#
# (Introduced in PR #404. expected output follows the __END__ separator.)
def try(name)
  s = yield
  l = s.is_a?(Array) ? s : [s]
  puts "#{name}: #{l.map { |x| [x.encoding.to_s, x.valid_encoding?] }.inspect}"
rescue Encoding::CompatibilityError => e
  puts "#{name}: #{e.message}"
end
try("delete_suffix") { "aé".delete_suffix(b"\xa9") }
try("delete_prefix") { "éa".delete_prefix(b"\xc3") }
try("sub") { "aé".sub(b"\xa9", "") }
try("gsub") { "abc".gsub("b", b"\xff") }
try("split") { "éa".split(b"\xa9") }
try("partition") { "éa".partition(b"\xa9") }
try("chomp") { "aé".chomp(b"\xa9") }
try("tr") { "abc".tr("b", b"\xff") }
try("center") { "abc".center(9, b"\xff") }
try("slice") { "aé"[b"\xa9"] }
try("%") { "%s" % b"\xff" }
try("% list") { "%s %s" % ["a", b"\xff"] }
try("format") { format("%s-%s", "é", b"\xff") }
try("ascii receiver") { "abc".force_encoding("US-ASCII").sub("b", b"\xff") }
try("ascii argument") { "abc".gsub("b", b"x") }
try("join") { ["a", "b"].join(b"\xff") }
try("join set") { Set[1, 2].join(b"\xff") }
try("list * string") { ["a", "b"] * b"\xff" }
try("prepend") { x = "abc"; x = x.prepend(b"\xff"); x }
try("insert") { x = "abc"; x = x.insert(1, b"\xff"); x }
try("insert in bytes") { x = b"\xff\xfe"; x = x.insert(1, "a"); x }
try("prepend é") { x = "é"; x = x.prepend(b"\xff"); x }
x = "héllo"
x = x.insert(2, "-")
x = x.prepend("a", "b")
p x

__END__
delete_suffix: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
delete_prefix: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
sub: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
gsub: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
split: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
partition: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
chomp: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
tr: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
center: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
slice: incompatible character encodings: UTF-8 and BINARY (ASCII-8BIT)
%: format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
% list: format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format: format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
ascii receiver: incompatible character encodings: US-ASCII and BINARY (ASCII-8BIT)
ascii argument: [["UTF-8", true]]
join: [["ASCII-8BIT", true]]
join set: [["ASCII-8BIT", true]]
list * string: [["ASCII-8BIT", true]]
prepend: [["ASCII-8BIT", true]]
insert: [["ASCII-8BIT", true]]
insert in bytes: [["ASCII-8BIT", true]]
prepend é: [["ASCII-8BIT", true]]
"abhé-llo"
