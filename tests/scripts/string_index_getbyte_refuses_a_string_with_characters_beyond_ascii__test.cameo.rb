# string index: testcase to demonstrate that getbyte refuses a string with
# characters beyond ASCII
#
# getbyte reads a byte of a string whose bytes are its characters, an ASCII
# one, or of a binary string. A UTF-8 string with characters beyond ASCII is
# refused, where Ruby returns a part of a character.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
p "abc".getbyte(0), "abc".getbyte(-1), "abc".getbyte(3)
p "é".b.getbyte(0), b"\xFFa".getbyte(1), "abc".force_encoding("US-ASCII").getbyte(1)
p format("%s", "abc").getbyte(1), "#{:a}b".getbyte(1)
["é", "aé", format("%s", "é"), "#{:é}"].each do |s|
  begin
    s.getbyte(0)
  rescue Encoding::CompatibilityError => e
    puts e.message
  end
end

__END__
97
99
nil
195
97
98
98
98
getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with [] (see https://15b.it/bin)
getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with [] (see https://15b.it/bin)
getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with [] (see https://15b.it/bin)
getbyte reads a byte, and this UTF-8 string has characters beyond ASCII: call b first for its bytes, or read a character with [] (see https://15b.it/bin)
