# binary joins: testcase to demonstrate that format refuses the binary string
# it puts in
#
# format refuses a binary String that a %s puts into the text, and the format
# itself. One that is not put in is not looked at: an argument no directive
# takes, and an element of a list or a hash that is inspected.
#
# (Introduced in PR #439. expected output follows the __END__ separator.)
def try
  yield
rescue Encoding::CompatibilityError => e
  puts "refused"
end
p format("%d", 1, b"ab"), "%p" % [[b"ab"]], format("%s", [b"ab"]), "%s" % { a: b"x" }
try { format("%s", b"ab") }
try { "%s %s" % ["a", b"ab"] }
try { format(b"%s", 1) }
try { format("%-5s|", b"\xFF") }

__END__
"1"
"[\"ab\"]"
"[\"ab\"]"
"{a: \"x\"}"
refused
refused
refused
refused
