package eval

import (
	"os"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The instructions that hold lists run them on cursors. The script goes
// through each with the orders that are easy to get wrong: a part of a
// string made a string before the next part is evaluated, an ensure
// clause around next, break, return and an exception, retry, a rescue
// clause that raises, the guards of case/in, defined? of what raises.
// The expected output is what the evaluator printed while these
// instructions called it for their lists.
func TestListsRunOnCursors(t *testing.T) {
	src, err := os.ReadFile("testdata/cursors.cameo")
	assert.NoError(t, err)
	want, err := os.ReadFile("testdata/cursors.expected")
	assert.NoError(t, err)
	assert.True(t, len(want) > 300, "the expected output is %d bytes", len(want))
	prints(t, string(src), string(want))
}

// A completion that leaves a list cuts the value stack to where the
// instruction that takes it started, whatever the expression in flight
// had pushed.
func TestACompletionCutsTheValueStack(t *testing.T) {
	prints(t, `def pick(a, b, c) = [a, b, c]
i = 0
r = while true
  i += 1
  pick(1, 2, (if i < 3 then next else break i end))
end
p r
v = pick(:a, begin
  pick(1, 2, raise("x"))
rescue => e
  e.message
end, :c)
p v
w = [1, begin
  [2, (raise "y")]
ensure
  puts "cut"
end] rescue "rescued"
p w
`, "3\n[:a, \"x\", :c]\ncut\n\"rescued\"\n")
}

// A list an instruction runs for a pattern, inside Go, is an activation's
// too: a completion out of it reaches the clauses around.
func TestAnExpressionInAPatternRaisesThroughTheClausesAround(t *testing.T) {
	prints(t, `def limit = raise(ArgumentError, "no limit")
r = begin
  case 5
  in ^(limit) then "hit"
  end
rescue ArgumentError => e
  e.message
ensure
  puts "ensure"
end
p r
`, "ensure\n\"no limit\"\n")
}
