package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// minispec is cameo's own, so these outputs are its specification; the
// messages of the assertions are minitest 5.25's.

func TestMinispecReportsFailuresErrorsAndSkipsWithTheirLineAndExits1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", `
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
`, cameoio.Streams(&stdout, &stderr))
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

func TestMinispecAnswersOnlyWhenRequired(t *testing.T) {
	// A script that did not require minispec has no `it`, and is told
	// where it is.
	var stdout, stderr bytes.Buffer
	err := cameo.RunFromString("t.cameo", "it(\"x\") { }\n", cameoio.Streams(&stdout, &stderr))
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:1: undefined method 'it' for main: require \"minispec\" first (NoMethodError)")
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
  include Minispec::DSL
  include Minispec::Assertions
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
	err := cameo.RunFromString("t.cameo", src, cameoio.Streams(&stdout, &stderr))
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
