package eval

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A script prints the same with the activations of tail calls kept and
// without them: a method that was given a block stays for the block's
// break, and a proc's return after its method made a tail call raises
// either way.
func TestAScriptRunsTheSameWithTailFramesKept(t *testing.T) {
	src := `def helper(blk) = blk.call(1)
def each_thing(&blk)
  helper(blk)
end
p(each_thing { |x| break x + 4 })
def twice
  yield
  helper(proc { |x| x * 2 })
end
p(twice { 1 })
def g(pr) = pr.call
def f
  pr = proc { return 1 }
  g(pr)
end
begin
  p f
rescue LocalJumpError => e
  puts e.message
end
`
	for _, keep := range []int{0, 4, TailFramesKept} {
		prog, err := syntax.Parse("t.cameo", src)
		assert.NoError(t, err)
		var out bytes.Buffer
		in := New(&out, &out)
		in.KeepTailFrames(keep)
		assert.NoError(t, in.Run("t.cameo", prog))
		assert.Equals(t, out.String(), "5\n2\nunexpected return\n")
	}
}
