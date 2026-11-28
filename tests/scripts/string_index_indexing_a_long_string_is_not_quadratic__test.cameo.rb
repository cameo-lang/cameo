# string index: testcase to demonstrate that indexing a long string is not
# quadratic
#
# Indexing does not depend on the length of the string: reading every
# character of a long ASCII string by index makes as many steps as it has
# characters. Before, each index decoded the whole string, and this script ran
# for minutes.
#
# (Introduced in PR #436. expected output follows the __END__ separator.)
made = { "repeat" => "ab " * 100_000, "format" => format("%s", "ab " * 100_000), "interpolated" => "#{:x}#{"ab " * 100_000}", "binary" => ("ab " * 100_000).b }
made.each do |name, s|
  i = 0
  i += 1 until s[i].nil?
  j = 0
  j += 1 until s.getbyte(j).nil?
  puts "#{name}: #{i} #{j}"
end

__END__
repeat: 300000 300000
format: 300000 300000
interpolated: 300001 300001
binary: 300000 300000
