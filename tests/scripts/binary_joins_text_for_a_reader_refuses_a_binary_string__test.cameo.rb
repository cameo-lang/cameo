# binary joins: testcase to demonstrate that text for a reader refuses a
# binary string
#
# An interpolation and format make text for a reader, and refuse a binary
# String, as the format or among the arguments, where Ruby makes a String of
# them. A String that says what its bytes are goes in.
#
# (Introduced in PR #439. expected output follows the __END__ separator.)
def try
  yield
rescue Encoding::CompatibilityError => e
  puts e.message
end
bin = b"ab"
try { "v=#{bin}" }
try { "v=#{b"\xFF"}" }
try { format("%s", bin) }
try { "%s" % bin }
try { "%s %s" % ["a", bin] }
try { format(bin, 1) }
try { sprintf("%d %s", 1, b"\xFF") }
p "v=#{bin.force_encoding("UTF-8")}", format("%s", "é"), "#{1}#{:a}#{nil}"
puts bin

__END__
an interpolation makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
an interpolation makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
format makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
"v=ab"
"é"
"1a"
ab
