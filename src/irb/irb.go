// Package irb is cameo's irb as a library. A Session evaluates lines at the
// top level of one program, as Ruby's irb does, and a Terminal edits them as
// they are typed, on any byte stream: arrow keys, Ctrl-A, Ctrl-E, Ctrl-K,
// Ctrl-W and a history on the up and down arrows (golang.org/x/term). The
// cameo command runs both on its terminal; a program that embeds cameo runs
// them on a stream of its own, such as an app's terminal that has no tty.
package irb

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Session evaluates lines at the top level of a program: its variables,
// methods and classes are the program's. What the lines print goes to the
// program's streams.
type Session struct {
	mu      sync.Mutex
	prog    *cameo.Program
	pending strings.Builder
	n       int
}

// NewSession returns a session on prog, which has run already or will not
// run: a line is evaluated with Program.Eval.
func NewSession(prog *cameo.Program) *Session {
	return &Session{prog: prog, n: 1}
}

// Prompt returns the prompt for the next line: cameo(main):001> for a new
// expression, cameo(main):001* while one is incomplete.
func (s *Session) Prompt() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending.Len() > 0 {
		return fmt.Sprintf("cameo(main):%03d* ", s.n)
	}
	return fmt.Sprintf("cameo(main):%03d> ", s.n)
}

// Eval takes one line and returns its value, => value, or its error, each
// with a newline, and whether the line ended the session: exit, quit or
// exit!. A blank line, or one that leaves an expression incomplete, returns
// no output and waits for the next.
func (s *Session) Eval(line string) (output string, done bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending.Len() == 0 {
		switch strings.TrimSpace(line) {
		case "":
			return "", false
		case "exit", "quit", "exit!":
			return "", true
		}
	}
	s.pending.WriteString(line)
	s.pending.WriteString("\n")
	value, err := s.prog.Eval(s.pending.String())
	if incomplete(err) {
		return "", false
	}
	s.pending.Reset()
	s.n++
	if err != nil {
		return err.Error() + "\n", false
	}
	return "=> " + value + "\n", false
}

// Cancel drops an incomplete expression, as Ctrl-C does.
func (s *Session) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending.Reset()
}

// incomplete returns whether err is a parse error that more input would
// cure: the parser ran out of input.
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
