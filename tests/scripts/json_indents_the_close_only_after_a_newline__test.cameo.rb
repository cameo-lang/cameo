# json: testcase to demonstrate that JSON indents the close only after a
# newline
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
puts [1, {a: 2}].to_json(indent: "  ")

__END__
[  1,  {    "a":2}]
