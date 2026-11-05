package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
	cirb "github.com/cameo-lang/cameo/src/irb"
	"github.com/cameo-lang/cameo/src/readline"
	"github.com/cameo-lang/cameo/src/syntax"
	"golang.org/x/term"
)

// irb reads lines from stdin and evaluates each at the top level of one
// script, printing its value, as Ruby's irb does. With a script given, the
// script runs first and the lines see what it defined. A line that is not
// a whole expression yet waits for the next. exit, quit or the end of the
// input ends it. It returns the process exit status.
func irb(path string, allowIO []string, stdin io.Reader, stdout, stderr io.Writer) int {
	prog, status, ok := loadIRB(path, allowIO, stdout, stderr)
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
			return finishIRB(prog, stderr)
		}
		output, done := s.Eval(in.Text())
		if done {
			return finishIRB(prog, stderr)
		}
		fmt.Fprint(stdout, output)
	}
}

// finishIRB runs the at_exit blocks the session's lines registered, as the
// end of a script runs the script's, and returns the exit status: a
// handler's exit, 1 when a handler raised, 0 otherwise.
func finishIRB(prog *cameo.Program, stderr io.Writer) int {
	err := prog.Finish()
	if err == nil {
		return 0
	}
	if status, ok := cameo.ExitStatus(err); ok {
		return status
	}
	fmt.Fprintln(stderr, err)
	return 1
}

// giveTerminal gives prog the terminal of stdin and stdout, when both are
// one, for Cameo::Kernel::Readline to read lines from: in raw mode while a
// line is read, with a history kept in memory.
func giveTerminal(prog *cameo.Program) {
	in, out := int(os.Stdin.Fd()), int(os.Stdout.Fd())
	if !term.IsTerminal(in) || !term.IsTerminal(out) {
		return
	}
	e := readline.New(struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}, readline.RawMode(func() (func(), error) {
		state, err := term.MakeRaw(in)
		if err != nil {
			return nil, err
		}
		return func() { term.Restore(in, state) }, nil
	}))
	if cols, rows, err := term.GetSize(out); err == nil {
		e.SetSize(cols, rows)
	}
	prog.Terminal(e)
}

// irbOnTerminal is irb on the terminal of stdin and stdout, which it puts in
// raw mode for the line editor (package readline): arrow keys, a history
// kept in ~/.cameo_history, and Ctrl-C, which drops the line.
func irbOnTerminal(path string, allowIO []string, stdin, stdout *os.File) int {
	fd := int(stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return irb(path, allowIO, stdin, stdout, os.Stderr)
	}
	defer term.Restore(fd, state)
	t := cirb.NewTerminal(struct {
		io.Reader
		io.Writer
	}{stdin, stdout}, readline.HistoryFile(readline.DefaultHistoryFile()))
	if cols, rows, err := term.GetSize(int(stdout.Fd())); err == nil {
		t.SetSize(cols, rows)
	}
	prog, status, ok := loadIRB(path, allowIO, t, t)
	if !ok {
		return status
	}
	if err := t.Run(cirb.NewSession(prog)); err != nil {
		fmt.Fprintln(t, err)
		return 1
	}
	return finishIRB(prog, t)
}

// loadIRB loads an empty program, or the script at path and runs it, for
// irb, allowing the I/O allowIO names, as --io does for a script. ok is false when the session does not start: the script did not
// load, or it exited; status is then the process exit status.
func loadIRB(path string, allowIO []string, stdout, stderr io.Writer) (prog *cameo.Program, status int, ok bool) {
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
	prog.AllowHostIO(allowIO...)
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
func debugScript(path string, allowIO []string, pause bool, stdin io.Reader, stdout, stderr io.Writer) int {
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
	return runProgram(prog, allowIO, pause, stderr)
}
