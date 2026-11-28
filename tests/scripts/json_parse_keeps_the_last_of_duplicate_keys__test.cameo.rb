# json: testcase to demonstrate that JSON parse keeps the last of duplicate
# keys
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
p JSON.parse('{"a":1,"b":2,"a":3}')

__END__
{"a" => 3, "b" => 2}
