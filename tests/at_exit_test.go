package tests

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// The scripts are plain Ruby; what they print, and the status they end
// with, are Ruby 3.4's.

func TestAtExitRunsTheHandlersLastFirstWhenTheScriptEnds(t *testing.T) {
	out := runScript(t, `
at_exit { puts "first registered" }
at_exit { puts "second"; at_exit { puts "nested" } }
h = at_exit { puts "third" }
p h.class
puts "body"
`)
	assert.Equals(t, out, "Proc\nbody\nthird\nsecond\nnested\nfirst registered\n")
}

func TestAtExitRunsAfterExitAndAHandlersExitSetsTheStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
at_exit { puts "one"; exit 3 }
at_exit { puts "two" }
exit 5
`, &stdout, &stderr)
	status, ok := cameo.ExitStatus(err)
	assert.True(t, ok, "the run ended with %v", err)
	assert.Equals(t, status, 3)
	assert.Equals(t, stdout.String(), "two\none\n")
}

func TestAtExitReportsAHandlersExceptionAndRunsTheRest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
at_exit { puts "still runs" }
at_exit { raise "in handler" }
puts "body"
`, &stdout, &stderr)
	status, ok := cameo.ExitStatus(err)
	assert.True(t, ok, "the run ended with %v", err)
	assert.Equals(t, status, 1)
	assert.Equals(t, stdout.String(), "body\nstill runs\n")
	assert.Equals(t, stderr.String(), "t.cameo:3: in handler (RuntimeError)\n")
}

func TestAtExitRunsWhenTheScriptRaises(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
at_exit { puts "handler" }
puts "body"
raise ArgumentError, "boom"
`, &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:4: boom (ArgumentError)")
	assert.Equals(t, stdout.String(), "body\nhandler\n")
}

func TestAtExitWithoutABlockIsAnArgumentError(t *testing.T) {
	out := runScript(t, `
begin
  at_exit
rescue ArgumentError => e
  puts e.message
end
`)
	assert.Equals(t, out, "called without a block\n")
}

func TestAtExitFromAnAsyncCallRunsAtTheEndToo(t *testing.T) {
	out := runScript(t, `
include Cameo
class Job
  def run
    at_exit { puts "from the worker" }
    nil
  end
end
await async Job.new, :run
at_exit { puts "from the top level" }
puts "body"
`)
	assert.Equals(t, out, "body\nfrom the top level\nfrom the worker\n")
}

func TestAHandlersExitAfterTheScriptRaisedStillReportsTheException(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
at_exit { exit 4 }
raise ArgumentError, "boom"
`, &stdout, &stderr)
	status, ok := cameo.ExitStatus(err)
	assert.True(t, ok, "the run ended with %v", err)
	assert.Equals(t, status, 4)
	assert.Equals(t, stderr.String(), "t.cameo:3: boom (ArgumentError)\n")
}

func TestAHandlersExceptionAfterACleanExitEndsWithStatusOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
at_exit { raise "late" }
exit 0
`, &stdout, &stderr)
	status, ok := cameo.ExitStatus(err)
	assert.True(t, ok, "the run ended with %v", err)
	assert.Equals(t, status, 1)
	assert.Equals(t, stderr.String(), "t.cameo:2: late (RuntimeError)\n")
}

func TestAStopRunsNoHandler(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", `
at_exit { puts "handler" }
Test.stop
while true do end
puts "after"
`, &stdout, &stderr)
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	prog.Module("Test").Def("stop", func([]any, map[string]any, *cameo.Block) (any, error) {
		cancel()
		return nil, nil
	})
	err = prog.RunContext(ctx)
	assert.True(t, errors.Is(err, context.Canceled), "RunContext returned %v", err)
	assert.Equals(t, stdout.String(), "")
}

func TestAWorkersHandlerRunsWhenTheScriptRaises(t *testing.T) {
	// The run waits for the workers before the handlers run, so a block
	// a worker registers is run, after the worker's own output.
	var stdout, stderr bytes.Buffer
	err := cameo.Run("t.cameo", `
include Cameo
class Job
  def run
    at_exit { puts "cleanup" }
    puts "worked"
  end
end
async Job.new, :run
raise "main failed"
`, &stdout, &stderr)
	assert.NotNil(t, err)
	assert.Equals(t, err.Error(), "t.cameo:10: main failed (RuntimeError)")
	assert.Equals(t, stdout.String(), "worked\ncleanup\n")
}

func TestFinishRunsAHandlerACallRegistered(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", `
def later = at_exit { puts "from the call" }
at_exit { puts "from the script" }
`, &stdout, &stderr)
	assert.NoError(t, err)
	assert.NoError(t, prog.Run())
	assert.Equals(t, stdout.String(), "from the script\n")
	_, err = prog.Call(nil, "later")
	assert.NoError(t, err)
	assert.NoError(t, prog.Finish())
	assert.NoError(t, prog.Finish())
	assert.Equals(t, stdout.String(), "from the script\nfrom the call\n")
}
