# csv: testcase to demonstrate that send reaches the index assignment ACSV row
# defines
#
# send(:[]=) is the assignment's call on a class with its own []=, and returns
# the copy, as Ruby's returns the row; on a core value it is refused as the
# assignment is.
#
# The expected strings are Ruby 3.4's, with csv 3.3.2.
#
# (Introduced in PR #270. expected output follows the __END__ separator.)
require "csv"
r = CSV::Row.new(["a"], [1])
p r.send(:[]=, "b", 2)
p r
begin
  [1].send(:[]=, 0, 2)
rescue CameoError => e
  puts e.message
end

__END__
#<CSV::Row "a":1 "b":2>
#<CSV::Row "a":1>
send(:[]=): []= is not supported; use assignment, which copies (see https://15b.it/mut)
