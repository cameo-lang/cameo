# ascii string: testcase to demonstrate that an ASCII string holds seven bit
# characters alone
#
# A US-ASCII string holds 7-bit characters alone. Bytes past them are refused
# where a script says they are US-ASCII, unlike in Ruby, and a method that
# brings other characters in through an argument gives a UTF-8 string, as in
# Ruby.
#
# (Introduced in PR #404. expected output follows the __END__ separator.)
a = "abc".force_encoding("ASCII")
p a.encoding, a.length, a.upcase, a.upcase.encoding, (a + "é").encoding, Encoding::US_ASCII
begin
  "é".force_encoding("ASCII")
rescue Encoding::InvalidByteSequenceError => e
  puts e.message
end
tests = {
  "tr" => -> { a.tr("a", "é") }, "center" => -> { a.center(7, "é") }, "sub" => -> { a.sub("b", "é") },
  "gsub" => -> { a.gsub(/b/, "é") }, "*" => -> { a * 2 }, "ljust" => -> { a.ljust(5, "ü") },
  "interp" => -> { "#{a}é" }, "join" => -> { [a, "é"].join }, "%" => -> { a % [] },
  "delete_suffix" => -> { a.delete_suffix("é") }, "encode" => -> { "é".encode("US-ASCII") },
}
tests.each do |name, f|
  begin
    v = f.call
    puts "#{name}: #{v.inspect} #{v.encoding} valid=#{v.valid_encoding?}"
  rescue => e
    puts "#{name}: #{e.class}: #{e.message}"
  end
end

__END__
#<Encoding:US-ASCII>
3
"ABC"
#<Encoding:US-ASCII>
#<Encoding:UTF-8>
#<Encoding:US-ASCII>
invalid byte sequence in US-ASCII: keep the bytes in a binary string (see https://15b.it/inv)
tr: "ébc" UTF-8 valid=true
center: "ééabcéé" UTF-8 valid=true
sub: "aéc" UTF-8 valid=true
gsub: "aéc" UTF-8 valid=true
*: "abcabc" US-ASCII valid=true
ljust: "abcüü" UTF-8 valid=true
interp: "abcé" UTF-8 valid=true
join: "abcé" UTF-8 valid=true
%: "abc" US-ASCII valid=true
delete_suffix: "abc" US-ASCII valid=true
encode: Encoding::UndefinedConversionError: U+00E9 from UTF-8 to US-ASCII
