package cameo

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// A resumed script has the procs it held when it paused, each with the
// values it had captured, and runs the blocks at_exit registered before
// the pause.
func TestResumeBringsBackProcsAndAtExitBlocks(t *testing.T) {
	prog, out := pausing(t, `class Scale
  int :by
  def lambda = ->(x) { x * by }
  def self.unit = new(by: 1)
end
factor = 3
scale = ->(x) { x * factor }
fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }
triple = Scale.new(by: 3).lambda
maker = -> { Scale.unit }
TABLE = {"up" => :upcase.to_proc, "scale" => scale}
at_exit { puts "first registered, factor #{factor}" }
at_exit { puts "last registered" }
factor = 100
puts "before"
Test.pause
puts scale.(2), fact.(5), triple.(7), maker.call.by
puts TABLE["up"].call("x"), TABLE["scale"].(1)
puts scale.lambda?, scale.arity
`)
	err := prog.Run()
	assert.True(t, errors.Is(err, ErrPaused), "Run returned %v, want ErrPaused", err)
	assert.Equals(t, out.String(), "before\n")
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, strings.Join(dump.Missing, ";"), "")

	var resumed bytes.Buffer
	cont, err := Resume(dump.Data, &resumed, &resumed)
	assert.NoError(t, err)
	assert.NoError(t, cont.Run())
	assert.Equals(t, resumed.String(), "6\n120\n21\n1\nX\n3\ntrue\n1\nlast registered\nfirst registered, factor 3\n")

	text, err := InspectDump(dump.Data)
	assert.NoError(t, err)
	assert.Contains(t, text, "  scale = #<Proc:t.cameo:7 (lambda)>\n")
	assert.Contains(t, text, "at_exit: 2\n")
}

// A proc that cannot travel is left out of the dump with the reason, as
// any other value is.
func TestADumpNamesAProcItLeavesOut(t *testing.T) {
	prog, _ := pausing(t, `each = [1].each
reads = -> { each.next }
at_exit { each.next }
Test.pause
puts 1
`)
	err := prog.Run()
	assert.True(t, errors.Is(err, ErrPaused), "Run returned %v, want ErrPaused", err)
	dump, err := prog.Dump()
	assert.NoError(t, err)
	assert.Equals(t, strings.Join(dump.Missing, ";"),
		"each (an Enumerator cannot be serialised);"+
			"reads (a Proc that captured each, an Enumerator cannot be serialised);"+
			"an at_exit block (a Proc that captured each, an Enumerator cannot be serialised)")
}
