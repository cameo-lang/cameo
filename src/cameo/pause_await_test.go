package cameo

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// watched is a script's output that a test reads while the script runs
// on another goroutine.
type watched struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *watched) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *watched) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// A promise made by hand that nothing settled is in the dump as pending.
// The resumed run has it under its id: the host settles it, and the
// script that awaits it goes on.
func TestAPendingPromiseResumesPending(t *testing.T) {
	prog, out := pausing(t, `include Cameo
pr = promise
other = promise
resolve(other, "settled before")
puts "made #{pr}"
Test.pause
puts await(other), await(pr)
`)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "promises:\n  1 promise pending\n  2 promise = \"settled before\"\n"; !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it:\n%s", text, want)
	}
	var rest bytes.Buffer
	cont, err := Resume(dump.Data, &rest, &rest)
	if err != nil {
		t.Fatal(err)
	}
	pending := cont.PendingPromise(1)
	if pending == nil || cont.PendingPromise(9) != nil {
		t.Fatalf("PendingPromise(1) is %v, PendingPromise(9) %v", pending, cont.PendingPromise(9))
	}
	if err := pending.Resolve("settled by the host"); err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String()+rest.String(), "made 1\nsettled before\nsettled by the host\n"; got != want {
		t.Fatalf("the script printed %q, want %q", got, want)
	}
}

// A script that waits in an await pauses there, and the resumed run
// waits again: for the promises it had not got yet, with the results it
// had. Here the script waits for a promise only the host settles, so the
// pause can only come while it waits.
func TestPauseWhileAwaiting(t *testing.T) {
	const src = `include Cameo
class Work
  def double(x) = x * 2
end
def gather(w, pr)
  jobs = [async(w, :double, 1), pr, async(w, :double, 3)]
  total = await(jobs).sum
  "total #{total}"
end
pr = promise
puts "waiting"
puts gather(Work.new, pr) + "!"
puts await(pr)
`
	var out watched
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- prog.Run() }()
	// The script prints, then waits for the promise nothing settles.
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), "waiting\n"); {
		if time.Now().After(deadline) {
			t.Fatal("the script did not come to its await")
		}
		time.Sleep(time.Millisecond)
	}
	// Long enough for it to start its calls and come to the promise: a
	// pause that came earlier would stop it before an instruction, which
	// the check of the dump below would tell.
	time.Sleep(200 * time.Millisecond)
	prog.Pause()
	if err := <-done; !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	d := stackOf(t, dump)
	// The statement, and gather, which waits: it has the first job's
	// result and waits for the promise made by hand.
	if len(d.Stack) != 2 || d.Stack[1].Awaiting == nil {
		t.Fatalf("the run paused with %d activations, the last awaiting %v", len(d.Stack), d.Stack[len(d.Stack)-1].Awaiting)
	}
	if aw := d.Stack[1].Awaiting; aw.Next != 1 || len(aw.Results) != 1 || len(aw.Promises) != 3 {
		t.Fatalf("gather awaits %v", aw)
	}
	var rest bytes.Buffer
	cont, err := Resume(dump.Data, &rest, &rest)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		cont.PendingPromise(d.Stack[1].Awaiting.Promises[1]).Resolve(int64(10))
	}()
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := rest.String(), "total 18!\n10\n"; got != want {
		t.Fatalf("after the resume the script printed %q, want %q", got, want)
	}
}

// A run that paused while it awaited goes on waiting when it is run
// again.
func TestARunThatAwaitedGoesOnWaiting(t *testing.T) {
	var out watched
	prog, err := Load("t.cameo", "include Cameo\npr = promise\nputs \"waiting\"\nputs await(pr) + 1\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- prog.Run() }()
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), "waiting\n"); {
		if time.Now().After(deadline) {
			t.Fatal("the script did not come to its await")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	prog.Pause()
	if err := <-done; !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if err := prog.PendingPromise(1).Resolve(int64(41)); err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "waiting\n42\n"; got != want {
		t.Fatalf("the script printed %q, want %q", got, want)
	}
}
