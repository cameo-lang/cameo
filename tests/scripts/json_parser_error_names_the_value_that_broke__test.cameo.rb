# json: testcase to demonstrate that JSON::ParserError names the value that
# broke
#
# Where a script is plain Ruby, the expected output is Ruby 3.4's.
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
['{"a": {"b" 1}}', '[{"b" 1}]', '{"a": [1 2]}', "[1, /x]", "1 // no newline"].each do |src|
  JSON.parse(src)
rescue JSON::ParserError => e
  puts e.message
end

__END__
unexpected token at '{"a": {"b" 1}}'
unexpected token at '{"b" 1}]'
unexpected token at '2]}'
unexpected token at 'x]'
unexpected token at ''
