package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// output is a buffer the script writes to while the test reads it.
type output struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Write(p)
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

// waitFor waits until out holds text.
func waitFor(t *testing.T, out *output, text string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !strings.Contains(out.String(), text); {
		if time.Now().After(deadline) {
			t.Fatalf("the script did not print %q; it printed %q", text, out.String())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTheSumOfTheLinesTyped(t *testing.T) {
	var out bytes.Buffer
	prog, err := start(strings.NewReader("3\n4\n\nten\n1.5\n"), &out, &out)
	assert.NoError(t, err)
	assert.NoError(t, prog.Run())
	assert.Equals(t, out.String(), "1: total 3\n2: total 7\nnot a number: ten\n3: total 8.5\nthe sum of 3 numbers is 8.5\n")
}

// A run paused while it waits for a line goes on, in another program, with
// the sum it had and the next line typed there.
func TestARunPausedWhileItWaitsGoesOn(t *testing.T) {
	keys, typed := io.Pipe()
	var out output
	prog, err := start(keys, &out, &out)
	assert.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- prog.Run() }()
	io.WriteString(typed, "3\n4\n")
	waitFor(t, &out, "2: total 7\n")
	// Time for the script to ask for the next line and wait for it; a
	// pause before that would leave nothing pending, which the check of
	// the dump tells.
	time.Sleep(100 * time.Millisecond)
	prog.Pause()
	if err := <-done; !errors.Is(err, cameo.ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	assert.NoError(t, err)
	ids, err := pending(dump.Data)
	assert.NoError(t, err)
	assert.Equals(t, len(ids), 1)

	// The same state goes on twice, each time with lines of its own.
	for _, input := range []string{"5\nten\n", "100\n"} {
		var rest bytes.Buffer
		cont, err := resume(dump.Data, strings.NewReader(input), &rest, &rest)
		assert.NoError(t, err)
		assert.NoError(t, cont.Run())
		want := "3: total 12\nnot a number: ten\nthe sum of 3 numbers is 12\n"
		if input == "100\n" {
			want = "3: total 107\nthe sum of 3 numbers is 107\n"
		}
		assert.Equals(t, rest.String(), want)
	}
}
