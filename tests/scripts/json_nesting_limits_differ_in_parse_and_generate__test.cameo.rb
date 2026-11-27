# json: testcase to demonstrate that JSON nesting limits differ in parse and
# generate
#
# (Introduced in PR #105. expected output follows the __END__ separator.)
begin
  JSON.parse("[" * 101 + "]" * 101)
rescue JSON::NestingError => e
  puts e.message
end
begin
  JSON.generate([[[[1]]]], max_nesting: 3)
rescue JSON::NestingError => e
  puts e.message
end
p JSON.parse("[" * 101 + "]" * 101, max_nesting: false).flatten

__END__
nesting of 101 is too deep
nesting of 3 is too deep
[]
