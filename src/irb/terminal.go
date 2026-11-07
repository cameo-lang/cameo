package irb

import (
	"errors"
	"io"
	"sync"

	"github.com/cameo-lang/cameo/src/readline"
)

// Terminal runs a Session on a byte stream, editing its lines with a
// readline.Editor: it reads the keys typed from the stream, and writes the
// line being edited, and what Write is given, to it. The stream is what a
// terminal in raw mode is: the keys arrive as they are typed, and the screen
// wants \r\n.
type Terminal struct {
	editor *readline.Editor

	mu      sync.Mutex
	session *Session
}

// NewTerminal returns a terminal on rw, whose editor opts configure: where
// the history is kept, and how many lines of it.
func NewTerminal(rw io.ReadWriter, opts ...readline.Option) *Terminal {
	t := &Terminal{editor: readline.New(rw, opts...)}
	t.editor.SetCompleter(func(before string) (string, []string) {
		t.mu.Lock()
		s := t.session
		t.mu.Unlock()
		if s == nil {
			return "", nil
		}
		return s.Complete(before)
	})
	return t
}

// Write shows p above the line being edited, with \n as \r\n. A program
// whose streams are the terminal prints through it.
func (t *Terminal) Write(p []byte) (int, error) {
	return t.editor.Write(p)
}

// SetSize tells the editor the screen's size, which it wraps long lines at.
func (t *Terminal) SetSize(cols, rows int) {
	t.editor.SetSize(cols, rows)
}

// Run reads lines and evaluates them in s until the session ends: exit or
// quit, Ctrl-D on an empty line, or the end of the stream. Ctrl-C drops the
// line being typed, and an incomplete expression. It returns the stream's
// error, and nil when the session ended.
func (t *Terminal) Run(s *Session) error {
	t.mu.Lock()
	t.session = s
	t.mu.Unlock()
	for {
		line, err := t.editor.ReadLine(s.Prompt(), true)
		if err == io.EOF {
			t.editor.Write([]byte("\n"))
			return nil
		}
		if errors.Is(err, readline.ErrInterrupt) {
			s.Cancel()
			continue
		}
		if err != nil {
			return err
		}
		output, done := s.Eval(line)
		if done {
			return nil
		}
		t.editor.Write([]byte(output))
	}
}
