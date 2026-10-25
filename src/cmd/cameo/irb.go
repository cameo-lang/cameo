package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/syntax"
)

// irb reads lines from stdin and evaluates each at the top level of one
// script, printing its value, as Ruby's irb does. With a script given, the
// script runs first and the lines see what it defined. A line that is not
// a whole expression yet waits for the next. exit, quit or the end of the
// input ends it. It returns the process exit status.
func irb(path string, stdin io.Reader, stdout, stderr io.Writer) int {
	src := ""
	file := "(irb)"
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		src, file = string(data), path
	}
	prog, err := cameo.Load(file, src, stdout, stderr)
	if err != nil {
		var errs syntax.ErrorList
		if errors.As(err, &errs) {
			fmt.Fprintln(stderr, errs)
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if path != "" {
		if err := prog.Run(); err != nil {
			// The script's exit ends the session with its status, as
			// a script run on its own.
			if status, ok := cameo.ExitStatus(err); ok {
				return status
			}
			fmt.Fprintln(stderr, err)
		}
	}
	in := bufio.NewScanner(stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	var pending strings.Builder
	n := 0
	for {
		if pending.Len() == 0 {
			n++
			fmt.Fprintf(stdout, "cameo(main):%03d> ", n)
		} else {
			fmt.Fprintf(stdout, "cameo(main):%03d* ", n)
		}
		if !in.Scan() {
			fmt.Fprintln(stdout)
			return 0
		}
		line := in.Text()
		if pending.Len() == 0 {
			switch strings.TrimSpace(line) {
			case "":
				n--
				continue
			case "exit", "quit", "exit!":
				return 0
			}
		}
		pending.WriteString(line)
		pending.WriteString("\n")
		value, err := prog.Eval(pending.String())
		if incomplete(err) {
			continue
		}
		pending.Reset()
		if err != nil {
			fmt.Fprintln(stdout, err)
			continue
		}
		fmt.Fprintf(stdout, "=> %s\n", value)
	}
}

// incomplete reports a parse error that more input would cure: the
// parser ran out of input.
func incomplete(err error) bool {
	var errs syntax.ErrorList
	if !errors.As(err, &errs) {
		return false
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "(EOF)") {
			return true
		}
	}
	return false
}
