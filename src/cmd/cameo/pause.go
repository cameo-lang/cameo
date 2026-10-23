package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/syntax"
)

// statusPaused is the exit status of a script that paused: neither a run
// that ended, 0, nor one that failed, 1.
const statusPaused = 3

// runScript runs the script at path and returns the process exit status.
// With pause set, ^C pauses the script and writes its dump; see
// runProgram.
func runScript(path string, pause bool, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prog, err := cameo.Load(path, string(src), stdout, stderr)
	if err != nil {
		var errs syntax.ErrorList
		if errors.As(err, &errs) {
			fmt.Fprintln(stderr, errs)
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runProgram(prog, pause, stderr)
}

// resume continues the script the dump at path holds and returns the
// process exit status, as runScript does.
func resume(path string, pause bool, stdout, stderr io.Writer) int {
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
	return runProgram(prog, pause, stderr)
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
// top-level statement and writes its dump to a temporary file; a second
// ^C, while the statement still runs, ends the process. Without it, ^C
// ends the process as it does any other.
func runProgram(prog *cameo.Program, pause bool, stderr io.Writer) int {
	if pause {
		interrupts := make(chan os.Signal, 2)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
		go func() {
			<-interrupts
			prog.Pause()
			fmt.Fprintln(stderr, "cameo: pausing before the next top-level statement; press ^C again to end the script")
			<-interrupts
			os.Exit(130)
		}()
	}
	err := prog.Run()
	if errors.Is(err, cameo.ErrPaused) {
		return writeDump(prog, stderr)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// writeDump writes the paused program's dump to a temporary file and
// says where, and how to resume it.
func writeDump(prog *cameo.Program, stderr io.Writer) int {
	dump, err := prog.Dump()
	if err != nil {
		fmt.Fprintln(stderr, "cameo: cannot dump the script:", err)
		return 1
	}
	base := strings.TrimSuffix(filepath.Base(prog.File()), filepath.Ext(prog.File()))
	f, err := os.CreateTemp("", "cameo-"+base+"-*.dump")
	if err != nil {
		fmt.Fprintln(stderr, "cameo: cannot write the dump:", err)
		return 1
	}
	_, err = f.Write(dump.Data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(stderr, "cameo: cannot write the dump:", err)
		return 1
	}
	for _, missing := range dump.Missing {
		fmt.Fprintf(stderr, "cameo: %s is not in the dump\n", missing)
	}
	fmt.Fprintf(stderr, "cameo: paused before line %d of %s\ncameo: resume with: cameo resume %s\n", dump.Line, prog.File(), f.Name())
	return statusPaused
}
