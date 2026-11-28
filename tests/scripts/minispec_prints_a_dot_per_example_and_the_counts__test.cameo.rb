# minispec: testcase to demonstrate that minispec prints a dot per example and
# the counts
#
# minispec is cameo's own, so these outputs are its specification; the
# messages of the assertions are minitest 5.25's.
#
# (Introduced in PR #262. expected output follows the __END__ separator.)
require "minispec"
describe "Integer" do
  describe "+" do
    it "adds" do
      assert_equal 4, 2 + 2
      assert_operator 3, :>, 2
    end
  end
  it "is a kind of Numeric" do
    assert_kind_of Numeric, 1
  end
end
it "runs outside a describe" do
  assert true
end

__END__
...

3 runs, 4 assertions, 0 failures, 0 errors, 0 skips
