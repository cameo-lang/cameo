// Command sum adds up the numbers typed, one a line, in a cameo script.
// ^C pauses the script and writes its state, the sum so far included, to
// a file; given that file, the program continues the script where it
// paused. ^D ends the input and prints the sum.
//
//	go run ./examples/sum           # type numbers; ^C pauses, ^D ends
//	go run ./examples/sum <dump>    # continue a paused run
//
// The script reads lines through the module Keyboard, whose line returns
// a promise: the script awaits it, and a run that awaits pauses at once.
// The dump holds the promise as pending, and the program that resumes
// the run settles it with the next line typed.
package main

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameopb"
)

//go:embed sum.cameo
var script string

// keyboard reads lines from r on a goroutine of its own and gives them to
// the script one at a time.
type keyboard struct {
	prog  *cameo.Program
	lines chan string
}

func newKeyboard(prog *cameo.Program, r io.Reader) *keyboard {
	k := &keyboard{prog: prog, lines: make(chan string)}
	go func() {
		s := bufio.NewScanner(r)
		for s.Scan() {
			k.lines <- s.Text()
		}
		close(k.lines)
	}()
	prog.Module("Keyboard").Def("line", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
		promise := prog.Promise()
		k.settle(promise)
		return promise, nil
	})
	return k
}

// settle resolves promise with the next line, or with nil once the input
// has ended, without waiting for it.
func (k *keyboard) settle(promise *cameo.Promise) {
	go func() {
		if line, ok := <-k.lines; ok {
			promise.Resolve(line)
			return
		}
		promise.Resolve(nil)
	}()
}

// start loads the script, reading its lines from stdin.
func start(stdin io.Reader, stdout, stderr io.Writer) (*cameo.Program, error) {
	prog, err := cameo.Load("sum.cameo", script, stdout, stderr)
	if err != nil {
		return nil, err
	}
	newKeyboard(prog, stdin)
	return prog, nil
}

// resume makes a program of the state in data that goes on reading its
// lines from stdin. The state holds the script and its values and nothing
// of the host, so the program is given Keyboard again, and the line the
// script was waiting for is the next one typed.
func resume(data []byte, stdin io.Reader, stdout, stderr io.Writer) (*cameo.Program, error) {
	prog, err := cameo.Resume(data, stdout, stderr)
	if err != nil {
		return nil, err
	}
	k := newKeyboard(prog, stdin)
	ids, err := pending(data)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		k.settle(prog.PendingPromise(id))
	}
	return prog, nil
}

// pending returns the ids of the promises the dump in data holds
// unsettled: the line the script asked for when it paused, or none when
// it paused before it asked.
func pending(data []byte) ([]int64, error) {
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(data, d); err != nil {
		return nil, err
	}
	var ids []int64
	for _, p := range d.Promises {
		if p.Pending {
			ids = append(ids, p.Id)
		}
	}
	return ids, nil
}

// run runs prog until the script ends or ^C pauses it, and then writes
// its state to a temporary file and says how to continue.
func run(prog *cameo.Program, stderr io.Writer) error {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	go func() {
		<-interrupts
		// A second ^C, before the dump is written, ends the program.
		signal.Stop(interrupts)
		prog.Pause()
	}()
	err := prog.Run()
	if !errors.Is(err, cameo.ErrPaused) {
		return err
	}
	dump, err := prog.Dump()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "sum-*.dump")
	if err != nil {
		return err
	}
	if _, err := f.Write(dump.Data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "\npaused; go on with: go run ./examples/sum %s\n", f.Name())
	return nil
}

func main() {
	var prog *cameo.Program
	var err error
	switch len(os.Args) {
	case 1:
		prog, err = start(os.Stdin, os.Stdout, os.Stderr)
	case 2:
		var data []byte
		if data, err = os.ReadFile(os.Args[1]); err == nil {
			prog, err = resume(data, os.Stdin, os.Stdout, os.Stderr)
		}
	default:
		err = errors.New("usage: sum [dump]")
	}
	if err == nil {
		err = run(prog, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
