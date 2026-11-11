package cameo

import (
	"bytes"
	"errors"
	"testing"
)

// counting is a script whose worker asks for the pause from the middle
// of a loop, while the script waits for it.
const counting = `include Cameo
class Work
  def count(n)
    i = 0
    total = 0
    while i < n
      i += 1
      total += i
      Test.pause if i == 500
    end
    [total, i]
  end
  def doubled(xs) = xs.map { |x| x * 2 }
end
w = Work.new
job = async(w, :count, 1000)
other = async(w, :doubled, [1, 2, 3])
puts "started"
puts await(job).inspect
puts await(other).inspect
`

// A worker stops at a pause where it is, inside its call, and the dump
// holds its stack under the promise it settles. The resumed run gives
// it an interpreter and a goroutine, and the script's await gets the
// result of the call that went on.
func TestAWorkerPausesInsideItsCall(t *testing.T) {
	prog, out := pausing(t, counting)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
	d := stackOf(t, dump)
	frozen := false
	for _, w := range d.Workers {
		// The worker of the first promise: one activation, count's.
		if w.Promise == 1 && len(w.Stack) == 1 && w.Stack[0].Line == 6 {
			frozen = true
		}
	}
	if !frozen {
		t.Fatalf("the dump has no worker for promise 1 inside count: %v", d.Workers)
	}
	for _, p := range d.Promises {
		if p.Id == 1 && !p.Pending {
			t.Fatalf("promise 1 is settled in the dump: %v", p)
		}
	}
	var rest bytes.Buffer
	cont, err := Resume(dump.Data, &rest, &rest)
	if err != nil {
		t.Fatal(err)
	}
	cont.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) { return nil, nil })
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String()+rest.String(), whole(t, counting); got != want {
		t.Fatalf("paused and resumed the script printed\n%s\nand in one run\n%s", got, want)
	}
}

// A run that paused with a worker stopped goes on when it is run again:
// the worker goes on too.
func TestAStoppedWorkerGoesOnWithItsRun(t *testing.T) {
	prog, out := pausing(t, counting)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), whole(t, counting); got != want {
		t.Fatalf("paused and run again the script printed\n%s\nand in one run\n%s", got, want)
	}
}

// A script that waits where it cannot pause, inside the block of a
// builtin that is no stepper, needs the call it waits for: that worker
// does not stop, the script gets its result, and the run pauses when the
// builtin returned. The dump holds no worker.
func TestAWorkerSomethingNeedsRunsToItsEnd(t *testing.T) {
	const src = `include Cameo
class Work
  def count(n)
    i = 0
    while i < n
      i += 1
      Test.pause if i == 500
    end
    i
  end
end
job = async(Work.new, :count, 1000)
shown = "a".gsub("a") { await(job).to_s }
puts shown
puts "after"
`
	prog, out := pausing(t, src)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	d := stackOf(t, dump)
	if len(d.Workers) != 0 || len(d.Calls) != 0 {
		t.Fatalf("the dump holds %d workers and %d calls, want none", len(d.Workers), len(d.Calls))
	}
	var rest bytes.Buffer
	cont, err := Resume(dump.Data, &rest, &rest)
	if err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String()+rest.String(), whole(t, src); got != want {
		t.Fatalf("paused and resumed the script printed\n%s\nand in one run\n%s", got, want)
	}
}

// Workers that wait for workers stop too, each where it waits, and a call
// no worker took yet stays a call in the dump. The resumed run finishes
// them all.
func TestWorkersThatAwaitWorkersPause(t *testing.T) {
	const src = `include Cameo
class Tree
  def sum(depth)
    return leaf(depth) if depth == 0
    left = async(self, :sum, depth - 1)
    right = async(self, :sum, depth - 1)
    await(left) + await(right)
  end
  def leaf(depth)
    Test.pause
    1
  end
end
puts await(Tree.new, :sum, 3)
puts "done"
`
	sameAsWhole(t, src)
}
