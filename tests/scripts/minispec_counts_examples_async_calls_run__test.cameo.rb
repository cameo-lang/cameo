# minispec: testcase to demonstrate that minispec counts examples async calls
# run
#
# minispec is cameo's own, so these outputs are its specification; the
# messages of the assertions are minitest 5.25's.
#
# (Introduced in PR #262. expected output follows the __END__ separator.)
require "minispec"
include Cameo
class Spec
  include Minispec::DSL
  include Minispec::Assertions
  def run(n)
    it("example #{n}") { assert_equal n, n }
    nil
  end
end
s = Spec.new
await [async(s, :run, 1), async(s, :run, 2)]

__END__
..

2 runs, 2 assertions, 0 failures, 0 errors, 0 skips
