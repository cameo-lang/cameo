# json: testcase to demonstrate that JSON parse decodes a high surrogate with
# any following escape
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
p JSON.parse('"\\ud83d\\u0041"')
begin
  JSON.parse('"\\ude00"')
rescue JSON::ParserError => e
  puts e.message
end

__END__
"👁"
a string that is not UTF-8 at '"\ude00"'
