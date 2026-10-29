package irb

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/term"
)

// Terminal is a line editor on a byte stream: it reads the keys typed from
// the stream, and writes the line being edited, and what Write is given, to
// it. The stream is what a terminal in raw mode is: the keys arrive as they
// are typed, and the screen wants \r\n.
type Terminal struct {
	keys *keyReader
	out  io.Writer

	mu      sync.Mutex
	editor  *term.Terminal
	cols    int
	rows    int
	session *Session
}

// NewTerminal returns a line editor on rw.
func NewTerminal(rw io.ReadWriter) *Terminal {
	t := &Terminal{keys: &keyReader{r: rw}, out: rw}
	t.editor = t.newEditor(nil, "")
	return t
}

// newEditor returns an editor on t's stream with history and prompt, and the
// size t was given. t.mu is not held: the editor writes to the screen.
func (t *Terminal) newEditor(history term.History, prompt string) *term.Terminal {
	e := term.NewTerminal(struct {
		io.Reader
		io.Writer
	}{t.keys, t.out}, prompt)
	if history != nil {
		e.History = history
	}
	e.AutoCompleteCallback = func(line string, pos int, key rune) (string, int, bool) {
		if key != '\t' {
			return "", 0, false
		}
		return t.complete(e, line, pos)
	}
	t.mu.Lock()
	cols, rows := t.cols, t.rows
	t.mu.Unlock()
	if cols > 0 && rows > 0 {
		e.SetSize(cols, rows)
	}
	return e
}

func (t *Terminal) current() *term.Terminal {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.editor
}

// Write shows p above the line being edited, with \n as \r\n. A program
// whose streams are the terminal prints through it.
func (t *Terminal) Write(p []byte) (int, error) {
	return t.current().Write(p)
}

// SetSize tells the editor the screen's size, which it wraps long lines at.
func (t *Terminal) SetSize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	t.mu.Lock()
	t.cols, t.rows = cols, rows
	editor := t.editor
	t.mu.Unlock()
	editor.SetSize(cols, rows)
}

// Run reads lines and evaluates them in s until the session ends: exit or
// quit, Ctrl-D on an empty line, or the end of the stream. Ctrl-C drops the
// line being typed, and an incomplete expression. It returns the stream's
// error, and nil when the session ended.
func (t *Terminal) Run(s *Session) error {
	t.mu.Lock()
	t.session = s
	t.mu.Unlock()
	t.current().SetPrompt(s.Prompt())
	for {
		editor := t.current()
		line, err := editor.ReadLine()
		if err == io.EOF {
			if !t.keys.ctrlC.Swap(false) {
				editor.Write([]byte("\n"))
				return nil
			}
			// x/term ends the line it was editing on Ctrl-C and keeps
			// its text: a new editor starts an empty one.
			// Straight to the screen: the old editor would draw its line
			// again below it.
			s.Cancel()
			t.out.Write([]byte("^C\r\n"))
			next := t.newEditor(editor.History, s.Prompt())
			t.mu.Lock()
			t.editor = next
			t.mu.Unlock()
			continue
		}
		if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
			return err
		}
		output, done := s.Eval(line)
		if done {
			return nil
		}
		editor.Write([]byte(output))
		editor.SetPrompt(s.Prompt())
	}
}

// complete answers Tab with the cursor at pos in line: the one name the
// word before the cursor can be completed to, or the longest start the
// names share, or, when the word is that start already, the names listed
// above the line. A Tab that completes nothing changes nothing.
func (t *Terminal) complete(e *term.Terminal, line string, pos int) (string, int, bool) {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return line, pos, true
	}
	word, names := s.Complete(line[:pos])
	if len(names) == 0 {
		return line, pos, true
	}
	common := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, common) {
			common = common[:len(common)-1]
		}
	}
	if len(common) > len(word) {
		insert := common[len(word):]
		return line[:pos] + insert + line[pos:], pos + len(insert), true
	}
	e.Write([]byte(strings.Join(names, "  ") + "\n"))
	return line, pos, true
}

// keyReader is the stream's keys. It notes a Ctrl-C, which the editor ends
// the line on as it does on Ctrl-D, so that Run can tell the two apart, and
// ends a read at it: the keys after it go to the next read, which the new
// editor Run starts after a Ctrl-C makes, where the old one would keep them.
type keyReader struct {
	r     io.Reader
	ctrlC atomic.Bool
	rest  []byte
}

func (k *keyReader) Read(p []byte) (int, error) {
	if len(k.rest) == 0 {
		buf := make([]byte, len(p))
		n, err := k.r.Read(buf)
		if n == 0 {
			return 0, err
		}
		k.rest = buf[:n]
	}
	chunk := k.rest
	if i := bytes.IndexByte(chunk, 3); i >= 0 {
		chunk = chunk[:i+1]
		k.ctrlC.Store(true)
	}
	n := copy(p, chunk)
	k.rest = k.rest[n:]
	return n, nil
}
