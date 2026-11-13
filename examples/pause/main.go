// Command pause stops a running script, keeps its state as bytes, and
// continues it later in another interpreter, as a program would that
// stores the bytes in a database and goes on after a restart.
//
//	go run ./examples/pause
//
// The script counts through five steps and calls Host.reached after
// each. On step 3 the Go program asks the run to pause: Run returns
// ErrPaused, Dump gives the state, and Resume makes a new program of it
// that goes on with step 4.
package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
)

//go:embed steps.cameo
var script string

// host gives prog the module Host, whose reached calls stop with the
// step the script is at.
func host(prog *cameo.Program, stop func(step int64)) {
	prog.Module("Host").Def("reached", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("reached takes one argument, not %d", len(args))
		}
		step, ok := args[0].(int64)
		if !ok {
			return nil, fmt.Errorf("reached takes an Integer, not %T", args[0])
		}
		stop(step)
		return nil, nil
	})
}

// firstHalf runs the script until it has reached step, pauses it there
// and returns its state.
func firstHalf(step int64, stdout, stderr io.Writer) ([]byte, error) {
	prog, err := cameo.Load("steps.cameo", script, stdout, stderr)
	if err != nil {
		return nil, err
	}
	host(prog, func(at int64) {
		if at == step {
			// Pause may be called from any goroutine: a timer, a
			// signal handler, a request to shut down.
			prog.Pause()
		}
	})
	if err := prog.Run(); !errors.Is(err, cameo.ErrPaused) {
		return nil, fmt.Errorf("the script did not pause: %v", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(stdout, "paused before line %d\n", dump.Line)
	return dump.Data, nil
}

// secondHalf continues the run whose state is data, to its end.
func secondHalf(data []byte, stdout, stderr io.Writer) error {
	prog, err := cameo.Resume(data, stdout, stderr)
	if err != nil {
		return err
	}
	// The state holds the script and its values, and nothing of the
	// host: the module is given again.
	host(prog, func(int64) {})
	return prog.Run()
}

func main() {
	data, err := firstHalf(3, os.Stdout, os.Stderr)
	if err == nil {
		fmt.Println("the state is bytes now; a program would store them")
		err = secondHalf(data, os.Stdout, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
