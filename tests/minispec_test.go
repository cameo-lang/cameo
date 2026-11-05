package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// minispec is cameo's own, so these outputs are its specification; the
// messages of the assertions are minitest 5.25's.

func TestMinispecPrintsADotPerExampleAndTheCounts(t *testing.T) {
	out := runScript(t, `
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
`)
	assert.Equals(t, out, "...\n\n3 runs, 4 assertions, 0 failures, 0 errors, 0 skips\n")
}

func TestMinispecReportsFailuresErrorsAndSkipsWithTheirLineAndExits1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
require "minispec"
describe "Report" do
  it "fails" do
    assert_equal 1, 2
  end
  it "raises" do
    nil.foo
  end
  it "skips" do
    skip "later"
  end
  it "has no block"
  it "passes" do
    pass
  end
end
`, &stdout, &stderr)
	status, ok := cameo.ExitStatus(err)
	assert.True(t, ok, "the run ended with %v", err)
	assert.Equals(t, status, 1)
	assert.Equals(t, stdout.String(), `FESS.

  1) Failure:
Report#fails [t.cameo:5]:
Expected: 1
  Actual: 2

  2) Error:
Report#raises [t.cameo:8]:
NoMethodError: undefined method 'foo' for nil

  3) Skipped:
Report#skips [t.cameo:11]:
later

  4) Skipped:
Report#has no block [t.cameo]:
(no tests defined)

5 runs, 2 assertions, 1 failures, 1 errors, 2 skips
`)
	assert.Equals(t, stderr.String(), "")
}

func TestMinispecAssertionMessagesAreMinitests(t *testing.T) {
	out := runScript(t, `
require "minispec"
def failing
  yield
  puts "held"
rescue Minispec::Assertion => e
  puts e.message
end
failing { assert false }
failing { assert nil, "custom" }
failing { assert_equal "a", "b" }
failing { refute_equal 1, 1 }
failing { assert_nil 1 }
failing { refute_nil nil }
failing { assert_empty [1] }
failing { refute_empty [] }
failing { assert_includes [1, 2], 3 }
failing { refute_includes [1, 2], 2 }
failing { assert_instance_of String, 1 }
failing { refute_instance_of Integer, 1 }
failing { assert_kind_of String, 1 }
failing { refute_kind_of Numeric, 1 }
failing { assert_match(/x/, "abc") }
failing { assert_match "a.c", "abc" }
failing { refute_match(/b/, "abc") }
failing { assert_respond_to 1, :upcase }
failing { refute_respond_to 1, :succ }
failing { assert_predicate 1, :zero? }
failing { refute_predicate 0, :zero? }
failing { assert_operator 1, :>, 2 }
failing { refute_operator 1, :<, 2 }
failing { assert_in_delta 1.0, 1.1 }
failing { assert_in_delta 1.0, 1.5, 0.1, "close" }
failing { refute_in_delta 1.0, 1.0001 }
failing { flunk }
failing { flunk "no" }
failing { assert_equal 1, 1 }
`)
	assert.Equals(t, out, `Expected false to be truthy.
custom.
Expected nil to be truthy.
Expected: "a"
  Actual: "b"
Expected 1 to not be equal to 1.
Expected 1 to be nil.
Expected nil to not be nil.
Expected [1] to be empty.
Expected [] to not be empty.
Expected [1, 2] to include 3.
Expected [1, 2] to not include 2.
Expected 1 to be an instance of String, not Integer.
Expected 1 to not be an instance of Integer.
Expected 1 to be a kind of String, not Integer.
Expected 1 to not be a kind of Numeric.
Expected /x/ to match "abc".
Expected /a\.c/ to match "abc".
Expected /b/ to not match "abc".
Expected 1 (Integer) to respond to #upcase.
Expected 1 to not respond to succ.
Expected 1 to be zero?.
Expected 0 to not be zero?.
Expected 1 to be > 2.
Expected 1 to not be < 2.
Expected |1.0 - 1.1| (0.10000000000000009) to be <= 0.001.
close.
Expected |1.0 - 1.5| (0.5) to be <= 0.1.
Expected |1.0 - 1.0001| (9.999999999998899e-05) to not be <= 0.001.
Epic Fail!
no
held
`)
}

func TestMinispecAssertRaisesReturnsTheExceptionAndFailsOtherwise(t *testing.T) {
	out := runScript(t, `
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
`)
	assert.Equals(t, out, `ArgumentError
"bad"
TypeError
RuntimeError
ArgumentError expected but nothing was raised.
with message.
ArgumentError exception expected, not
Class: <TypeError>
Message: <"boom">
skip passes through: inside
`)
}

func TestMinispecCountsExamplesAsyncCallsRun(t *testing.T) {
	out := runScript(t, `
require "minispec"
include Cameo
class Spec
  def run(n)
    it("example #{n}") { assert_equal n, n }
    nil
  end
end
s = Spec.new
await [async(s, :run, 1), async(s, :run, 2)]
`)
	assert.Equals(t, out, "..\n\n2 runs, 2 assertions, 0 failures, 0 errors, 0 skips\n")
}

func TestMinispecAssertRaisesCapturesAnExpectedAssertion(t *testing.T) {
	out := runScript(t, `
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
`)
	assert.Equals(t, out, "Expected: 1\n  Actual: 2\nlater\nraised again: not expected\n")
}

func TestMinispecReportsADescribeWithoutExamples(t *testing.T) {
	out := runScript(t, `
require "minispec"
describe "nothing yet" do
end
`)
	assert.Equals(t, out, "\n\n0 runs, 0 assertions, 0 failures, 0 errors, 0 skips\n")
}

func TestMinispecAnswersOnlyWhenRequired(t *testing.T) {
	// The functions are methods of every object, as minitest's are, but
	// a script that did not require minispec has no `it`.
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", "it(\"x\") { }\n", &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:1: undefined method 'it' for an instance of Object: require \"minispec\" first (NoMethodError)")
	assert.Equals(t, stdout.String(), "")
}

func TestMinispecBuildsAMessageOnFailureOnly(t *testing.T) {
	// A message Proc is called when the assertion failed, not before, as
	// minitest calls it; assert_operator with two values is a predicate.
	out, status := runSpec(t, `
require "minispec"
it "passes" do
  assert_equal 1, 1, -> { puts "not called" ; "msg" }
  assert_operator [], :empty?
  refute_operator [1], :empty?
end
it "fails" do
  assert_equal 1, 2, -> { puts "called" ; "msg" }
end
`)
	assert.Equals(t, status, 1)
	assert.Contains(t, out, ".called\nF")
	assert.NotContains(t, out, "not called")
	assert.Contains(t, out, "msg.\nExpected: 1\n  Actual: 2")
	assert.Contains(t, out, "2 runs, 4 assertions, 1 failures, 0 errors, 0 skips")
}

func TestMinispecNamesAWorkersExamplesByItsOwnDescribes(t *testing.T) {
	out, status := runSpec(t, `
require "minispec"
include Cameo
class Specs
  def run(name)
    describe name do
      it "fails" do
        assert false
      end
    end
  end
end
describe "outer" do
  await [async(Specs.new, :run, "a"), async(Specs.new, :run, "b")]
end
`)
	assert.Equals(t, status, 1)
	assert.Contains(t, out, "a#fails")
	assert.Contains(t, out, "b#fails")
	assert.NotContains(t, out, "outer::a")
	assert.Contains(t, out, "2 runs, 2 assertions, 2 failures, 0 errors, 0 skips")
}

// runSpec runs a spec script and returns its output and exit status.
func runSpec(t *testing.T, src string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", src, &stdout, &stderr)
	status := 0
	if err != nil {
		s, ok := cameo.ExitStatus(err)
		if !ok {
			t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
		}
		status = s
	}
	return stdout.String(), status
}
