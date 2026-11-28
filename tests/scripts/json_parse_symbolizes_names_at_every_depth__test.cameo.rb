# json: testcase to demonstrate that JSON parse symbolizes names at every
# depth
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
p JSON.parse('{"a": [{"b": 1}]}', symbolize_names: true)

__END__
{a: [{b: 1}]}
