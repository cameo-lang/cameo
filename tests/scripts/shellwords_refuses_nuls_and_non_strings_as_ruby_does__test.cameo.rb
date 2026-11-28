# shellwords: testcase to demonstrate that shellwords refuses NU ls and non
# strings as Ruby does
#
# A NUL is refused as shellwords 0.2.2 refuses it, what is not a String raises
# Ruby's NoMethodError, an unmatched quote names its position, and a long line
# is split in one pass.
#
# The expected strings are Ruby 3.4's, with shellwords 0.2.2.
#
# (Introduced in PR #296. expected output follows the __END__ separator.)
require "shellwords"
["a\0b", "x 'y", "a \"b"].each do |s|
  Shellwords.split(s)
rescue ArgumentError => e
  puts e.message.inspect
end
begin; Shellwords.escape("a\0b"); rescue ArgumentError => e; puts e.message; end
[nil, 1].each do |v|
  Shellwords.split(v)
rescue NoMethodError => e
  puts e.message
end
p Shellwords.split("w " * 20000).size

__END__
"Nul character at 1: ...\u0000"
"Unmatched quote at 2: ...'"
"Unmatched quote at 2: ...\""
NUL character
undefined method 'scan' for nil
undefined method 'scan' for an instance of Integer
20000
