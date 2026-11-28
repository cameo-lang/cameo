# valid string: testcase to demonstrate that a string of UTF8 holds whole
# characters alone
#
# A UTF-8 string is made of whole characters alone, unlike Ruby's. Bytes that
# are not UTF-8 are a binary string where they come in, as from pack or in a
# literal of their own, and calling them UTF-8 raises.
#
# (Introduced in PR #404. expected output follows the __END__ separator.)
p b"\xC3".encoding, "caf\xC3\xA9".encoding, "caf\xC3\xA9", b"a\xFFb".bytes, b"\xC3" + b"\xA9", (b"\xC3" + b"\xA9").force_encoding("UTF-8")
p [0xC3].pack("C").encoding, [233].pack("U").encoding, "é".unpack("a1"), "é".b.force_encoding("UTF-8"), "é".valid_encoding?
[-> { b"\xC3".force_encoding("UTF-8") }, -> { b"\xFF".force_encoding("US-ASCII") }].each do |f|
  begin
    p f.call
  rescue Encoding::InvalidByteSequenceError => e
    puts e.message
  end
end

__END__
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
"café"
[97, 255, 98]
"\xC3\xA9"
"é"
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:UTF-8>
["\xC3"]
"é"
true
invalid byte sequence in UTF-8: keep the bytes in a binary string (see https://15b.it/inv)
invalid byte sequence in US-ASCII: keep the bytes in a binary string (see https://15b.it/inv)
