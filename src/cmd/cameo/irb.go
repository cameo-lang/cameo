package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
	cirb "github.com/cameo-lang/cameo/src/irb"
	"github.com/cameo-lang/cameo/src/syntax"
	"golang.org/x/term"
)

// irb reads lines from stdin and evaluates each at the top level of one
// script, printing its value, as Ruby's irb does. With a script given, the
// script runs first and the lines see what it defined. A line that is not
// a whole expression yet waits for the next. exit, quit or the end of the
// input ends it. It returns the process exit status.
func irb(path string, stdin io.Reader, stdout, stderr io.Writer) int {
	prog, status, ok := loadIRB(path, stdout, stderr)
	if !ok {
		return status
	}
	s := cirb.NewSession(prog)
	in := bufio.NewScanner(stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	for {
		fmt.Fprint(stdout, s.Prompt())
		if !in.Scan() {
			fmt.Fprintln(stdout)
			return 0
		}
		output, done := s.Eval(in.Text())
		if done {
			return 0
		}
		fmt.Fprint(stdout, output)
	}
}

// irbOnTerminal is irb on the terminal of stdin and stdout, which it puts in
// raw mode for the line editor (package irb): arrow keys, history and
// Ctrl-C, which drops the line.
func irbOnTerminal(path string, stdin, stdout *os.File) int {
	fd := int(stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return irb(path, stdin, stdout, os.Stderr)
	}
	defer term.Restore(fd, state)
	t := cirb.NewTerminal(struct {
		io.Reader
		io.Writer
	}{stdin, stdout})
	if cols, rows, err := term.GetSize(int(stdout.Fd())); err == nil {
		t.SetSize(cols, rows)
	}
	prog, status, ok := loadIRB(path, t, t)
	if !ok {
		return status
	}
	if err := t.Run(cirb.NewSession(prog)); err != nil {
		fmt.Fprintln(t, err)
		return 1
	}
	return 0
}

// loadIRB loads an empty program, or the script at path and runs it, for
// irb. ok is false when the session does not start: the script did not
// load, or it exited; status is then the process exit status.
func loadIRB(path string, stdout, stderr io.Writer) (prog *cameo.Program, status int, ok bool) {
	src := ""
	file := "(irb)"
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return nil, 1, false
		}
		src, file = string(data), path
	}
	prog, err := cameo.Load(file, src, stdout, stderr)
	if err != nil {
		var errs syntax.ErrorList
		if errors.As(err, &errs) {
			fmt.Fprintln(stderr, errs)
			return nil, 1, false
		}
		fmt.Fprintln(stderr, err)
		return nil, 1, false
	}
	if path != "" {
		if err := prog.Run(); err != nil {
			// The script's exit ends the session with its status, as
			// a script run on its own.
			if status, ok := cameo.ExitStatus(err); ok {
				return nil, status, false
			}
			fmt.Fprintln(stderr, err)
		}
	}
	return prog, 0, true
}

// debugScript runs the script at path under the debugger, which stops before
// its first statement and talks on stdin and stdout; see doc/debugger.md.
// It returns the process exit status: the script's, or 0 when the user
// quit the debugger.
func debugScript(path string, pause bool, stdin io.Reader, stdout, stderr io.Writer) int {
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
	prog.Debug(stdin, stdout)
	return runProgram(prog, pause, stderr)
}
