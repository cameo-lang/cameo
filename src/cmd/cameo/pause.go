package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/syntax"
)

// statusPaused is the exit status of a script that paused: neither a run
// that ended, 0, nor one that failed, 1.
const statusPaused = 3

// runScript runs the script at path and returns the process exit status.
// With pause set, ^C pauses the script and writes its dump; see
// runProgram. With ignoreMutations set, a call of one of Ruby's mutators
// returns its result on a copy instead of raising. With keepTail set, a
// call that makes a tail call stays on the stack.
func runScript(path string, argv, allowIO []string, pause, ignoreMutations, keepTail bool, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// An argument is text for the script.
	for _, a := range argv {
		if !utf8.ValidString(a) {
			fmt.Fprintf(stderr, "cameo: the argument %q is not UTF-8\n", a)
			return 2
		}
	}
	prog, err := cameo.Load(path, string(src), stdout, stderr)
	if err == nil {
		prog.Args(argv)
	}
	if err != nil {
		var errs syntax.ErrorList
		if errors.As(err, &errs) {
			fmt.Fprintln(stderr, errs)
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog.IgnoreMutationErrors(ignoreMutations)
	if keepTail {
		prog.KeepTailFrames(cameo.TailFramesKept)
	}
	// A script that requires "debug" talks to the debugger on the
	// terminal.
	prog.Console(os.Stdin, stdout)
	giveTerminal(prog)
	return runProgram(prog, allowIO, pause, stderr)
}

// resume continues the script the dump at path holds and returns the
// process exit status, as runScript does.
func resume(path string, allowIO []string, pause, ignoreMutations, keepTail bool, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog, err := cameo.Resume(data, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog.IgnoreMutationErrors(ignoreMutations)
	if keepTail {
		prog.KeepTailFrames(cameo.TailFramesKept)
	}
	giveTerminal(prog)
	return runProgram(prog, allowIO, pause, stderr)
}

// inspect prints what the dump at path holds.
func inspect(path string, out io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, err := cameo.InspectDump(data)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

// runProgram runs a loaded or resumed program and returns the process
// exit status: 0 when it ended, 1 when it failed, statusPaused when it
// paused. With pause set, the first ^C pauses the script before its next
// instruction, or once the builtin it is inside has returned, and writes
// its dump to a temporary file; a second ^C, while the script still
// runs, ends the process. Without it, ^C ends the process as it does any
// other.
func runProgram(prog *cameo.Program, allowIO []string, pause bool, stderr io.Writer) int {
	// The command is the host: a script it runs may do the I/O --io named.
	prog.AllowHostIO(allowIO...)
	// pausing is set from the ^C that pauses the script until its dump is
	// written, or it goes on because it has none.
	var pausing atomic.Bool
	if pause {
		interrupts := make(chan os.Signal, 2)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
		go func() {
			for range interrupts {
				if pausing.Swap(true) {
					os.Exit(130)
				}
				// The message goes out before the pause is asked for: the
				// script may stop, and say where or that it cannot, as
				// soon as it is asked.
				fmt.Fprintln(stderr, "cameo: pausing; press ^C again to end the script")
				prog.Pause()
			}
		}()
	}
	err := prog.Run()
	for errors.Is(err, cameo.ErrPaused) {
		if status, written := writeDump(prog, stderr); written {
			return status
		}
		// The script goes on, and a ^C pauses it again.
		pausing.Store(false)
		err = prog.Run()
	}
	if status, ok := cameo.ExitStatus(err); ok {
		return status
	}
	if err != nil {
		// The debugger's quit ends the run, which is not the script
		// failing.
		if prog.Quit() {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// writeDump writes the paused program's dump to a temporary file and
// says where, and how to resume it. It returns written unset when there
// is no dump, since the script cannot be dumped where it paused or the
// file cannot be written: the script then goes on.
func writeDump(prog *cameo.Program, stderr io.Writer) (status int, written bool) {
	dump, err := prog.Dump()
	if err != nil {
		fmt.Fprintf(stderr, "cameo: cannot dump the script: %v\ncameo: the script goes on\n", err)
		return 0, false
	}
	base := strings.TrimSuffix(filepath.Base(prog.File()), filepath.Ext(prog.File()))
	f, err := os.CreateTemp("", "cameo-"+base+"-*.dump")
	if err == nil {
		_, err = f.Write(dump.Data)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "cameo: cannot write the dump: %v\ncameo: the script goes on\n", err)
		return 0, false
	}
	for _, missing := range dump.Missing {
		fmt.Fprintf(stderr, "cameo: %s is not in the dump\n", missing)
	}
	fmt.Fprintf(stderr, "cameo: paused before line %d of %s\ncameo: resume with: cameo resume %s\n", dump.Line, prog.File(), f.Name())
	return statusPaused, true
}
