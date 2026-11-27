# minispec: testcase to demonstrate that minispec assert raises captures an
# expected assertion
#
# minispec is cameo's own, so these outputs are its specification; the
# messages of the assertions are minitest 5.25's.
#
# (Introduced in PR #262. expected output follows the __END__ separator.)
require "minispec"
e = assert_raises(Minispec::Assertion) { assert_equal 1, 2 }
puts e.message
e = assert_raises(Minispec::Skip) { skip "later" }
puts e.message
begin
  assert_raises(ArgumentError) { flunk "not expected" }
rescue Minispec::Assertion => e
  puts "raised again: #{e.message}"
end

__END__
Expected: 1
  Actual: 2
later
raised again: not expected
