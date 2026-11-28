# minispec: testcase to demonstrate that minispec assert raises returns the
# exception and fails otherwise
#
# minispec is cameo's own, so these outputs are its specification; the
# messages of the assertions are minitest 5.25's.
#
# (Introduced in PR #262. expected output follows the __END__ separator.)
require "minispec"
e = assert_raises(ArgumentError) { raise ArgumentError, "bad" }
p e.class, e.message
e = assert_raises(TypeError, ArgumentError) { raise TypeError, "wrong" }
p e.class
e = assert_raises { raise "plain" }
p e.class
begin
  assert_raises(ArgumentError) { 1 }
rescue Minispec::Assertion => e
  puts e.message
end
begin
  assert_raises(ArgumentError, "with message") { raise TypeError, "boom" }
rescue Minispec::Assertion => e
  puts e.message
end
begin
  assert_raises(ArgumentError) { skip "inside" }
rescue Minispec::Skip => e
  puts "skip passes through: #{e.message}"
end

__END__
ArgumentError
"bad"
TypeError
RuntimeError
ArgumentError expected but nothing was raised.
with message.
ArgumentError exception expected, not
Class: <TypeError>
Message: <"boom">
skip passes through: inside
