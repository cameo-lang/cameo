# binary string: testcase to demonstrate that a binary string has no
# characters
#
# A method that needs to know what character a byte is raises for a binary
# string, where Ruby reads the bytes as ASCII; the same bytes said to be UTF-8
# answer it.
#
# (Introduced in PR #403. expected output follows the __END__ separator.)
b = "caf\xC3\xA9 1".b
%w[upcase downcase capitalize swapcase chars each_char reverse strip lstrip chomp chop to_i to_f hex ord succ
   split lines squeeze to_sym codepoints].each do |name|
  begin
    b.send(name)
    puts "#{name} answered"
  rescue Encoding::CompatibilityError => e
    puts e.message unless e.message.start_with?("#{name} reads characters")
  end
end
[-> { b.center(9) }, -> { b.tr("a", "b") }, -> { b.count("a") }, -> { b % [] }, -> { b.casecmp("x") }].each do |f|
  begin
    f.call
    puts "answered"
  rescue Encoding::CompatibilityError
  end
end
t = b.force_encoding("UTF-8")
p t.upcase, t.chars.size, t.reverse, t.split, "abc".force_encoding("US-ASCII").upcase

__END__
"CAFÉ 1"
6
"1 éfac"
["café", "1"]
"ABC"
