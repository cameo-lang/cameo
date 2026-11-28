# time strptime space: testcase to demonstrate that time strptime reads epoch
# seconds before whitespace
#
# Time.strptime reads %s from a string with whitespace after it, such as a
# line of a file with its newline, as CRuby does.
#
# (Introduced in PR #366. expected output follows the __END__ separator.)
require "time"
p Time.strptime("1700000000\n", "%s").to_i
p Time.strptime("1700000000 \t", "%s").to_i
p Time.strptime("1700000000.5\n", "%s.%N").to_i

__END__
1700000000
1700000000
1700000000
