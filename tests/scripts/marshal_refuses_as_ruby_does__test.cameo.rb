# marshal: testcase to demonstrate that marshal refuses as Ruby does
#
# A refusal names the class of the value that cannot be dumped, wherever it is
# nested, in Ruby's words; the depth limit counts as Ruby's does; empty data
# is nil, nil's envelope.
#
# (Introduced in PR #271. expected output follows the __END__ separator.)
require "marshal"
require "date"
require "uri"
[[-> { 1 }], {"k" => -> {}}, Hash.new { |h, k| 0 }, Date.new(2020, 1, 2), URI("http://x.y/")].each do |v|
  Marshal.dump(v)
rescue TypeError => e
  puts e.message
end
p Marshal.load("")
begin; Marshal.dump(1, foo: 1); rescue TypeError => e; puts e.message; end
[[1, 0], [1, 1], [[1], 1], [[1], 2], [[[1]], 2], [{"a" => [1]}, 2], [{"a" => [1]}, 3], [1, -1]].each do |v, limit|
  Marshal.dump(v, limit)
  puts "ok #{limit}"
rescue ArgumentError => e
  puts "#{e.message} #{limit}"
end

__END__
no _dump_data is defined for class Proc
no _dump_data is defined for class Proc
can't dump hash with default proc
no _dump_data is defined for class Date
no _dump_data is defined for class URI::HTTP
nil
Marshal.dump writes a String: an IO is not supported
exceed depth limit 0
ok 1
exceed depth limit 1
ok 2
exceed depth limit 2
exceed depth limit 2
ok 3
ok -1
