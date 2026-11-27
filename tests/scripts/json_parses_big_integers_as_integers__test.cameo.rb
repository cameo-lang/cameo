# json: testcase to demonstrate that JSON parses big integers as integers
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
p JSON.parse("[123456789012345678901234567890, -9223372036854775809]").map(&:class)

__END__
[Integer, Integer]
