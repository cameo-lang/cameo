// Package readline is cameo's line editor: an Editor reads lines typed on
// any byte stream, with arrow keys, Ctrl-A, Ctrl-E, Ctrl-K, Ctrl-W, the Alt
// chords, Tab completion and a history on the up and down arrows
// (golang.org/x/term). The history holds a limited number of lines and, when
// the host names a file, is kept in it between runs. irb edits its lines with
// an Editor, and a script reads lines from the one its host gave it through
// Cameo::Kernel::Readline (Install).
package readline

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/term"
)

// DefaultHistoryLimit is the number of lines a history keeps unless
// HistoryLimit says otherwise: the newest, as Ruby's irb keeps 1000.
const DefaultHistoryLimit = 1000

// ErrInterrupt is what ReadLine returns when Ctrl-C ends the line.
var ErrInterrupt = errors.New("readline: interrupted")

// DefaultHistoryFile returns the history file the cameo command keeps:
// .cameo_history in the user's home directory, or "" where there is none.
func DefaultHistoryFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".cameo_history")
}

// Option configures an Editor.
type Option func(*Editor)

// HistoryFile keeps the history in the file at path: New reads it, and each
// line added rewrites it with the newest lines the limit allows. The history
// is a convenience, so a file that cannot be read or written leaves it in
// memory. An empty path, the default, keeps it in memory only.
func HistoryFile(path string) Option {
	return func(e *Editor) { e.history.path = path }
}

// HistoryLimit keeps the newest n lines; n below 1 keeps
// DefaultHistoryLimit.
func HistoryLimit(n int) Option {
	return func(e *Editor) {
		if n >= 1 {
			e.history.limit = n
		}
	}
}

// RawMode puts the terminal in raw mode for each ReadLine: enter is called
// before the line is read, and the function it returns after. A host whose
// terminal is in raw mode already, as irb's is, does not need it.
func RawMode(enter func() (restore func(), err error)) Option {
	return func(e *Editor) { e.raw = enter }
}

// Editor reads lines from a byte stream and writes the line being edited,
// and what Write is given, to it. The stream is what a terminal in raw mode
// is: the keys arrive as they are typed, and the screen wants \r\n.
type Editor struct {
	keys    *keyReader
	out     io.Writer
	raw     func() (func(), error)
	history *history

	mu       sync.Mutex
	term     *term.Terminal
	cols     int
	rows     int
	complete func(before string) (word string, names []string)
}

// New returns an editor on rw, with the history file read.
func New(rw io.ReadWriter, opts ...Option) *Editor {
	e := &Editor{keys: &keyReader{r: rw}, out: rw, history: &history{limit: DefaultHistoryLimit}}
	for _, o := range opts {
		o(e)
	}
	e.history.load()
	e.term = e.newTerm()
	return e
}

// SetCompleter makes Tab complete the line: complete returns the word being
// typed at the end of before, the line up to the cursor, and the names it
// can be completed to.
func (e *Editor) SetCompleter(complete func(before string) (word string, names []string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.complete = complete
}

// newTerm returns an x/term editor on e's stream with e's history and
// size. e.mu is not held: the editor writes to the screen.
func (e *Editor) newTerm() *term.Terminal {
	t := term.NewTerminal(struct {
		io.Reader
		io.Writer
	}{e.keys, e.out}, "")
	t.History = e.history
	t.AutoCompleteCallback = func(line string, pos int, key rune) (string, int, bool) {
		if key != '\t' {
			return "", 0, false
		}
		return e.tab(t, line, pos)
	}
	e.mu.Lock()
	cols, rows := e.cols, e.rows
	e.mu.Unlock()
	if cols > 0 && rows > 0 {
		t.SetSize(cols, rows)
	}
	return t
}

func (e *Editor) current() *term.Terminal {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.term
}

// Write shows p above the line being edited, with \n as \r\n.
func (e *Editor) Write(p []byte) (int, error) {
	return e.current().Write(p)
}

// SetSize tells the editor the screen's size, which it wraps long lines at.
func (e *Editor) SetSize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	e.mu.Lock()
	e.cols, e.rows = cols, rows
	t := e.term
	e.mu.Unlock()
	t.SetSize(cols, rows)
}

// ReadLine shows prompt and returns the line typed, without its newline,
// adding it to the history when addHistory is true. It returns io.EOF at
// Ctrl-D on an empty line or the end of the stream, ErrInterrupt at Ctrl-C,
// which drops the line, and the stream's error otherwise.
func (e *Editor) ReadLine(prompt string, addHistory bool) (string, error) {
	if e.raw != nil {
		restore, err := e.raw()
		if err != nil {
			return "", err
		}
		defer restore()
	}
	t := e.current()
	t.SetPrompt(prompt)
	e.history.setAdding(addHistory)
	line, err := t.ReadLine()
	e.history.setAdding(false)
	if err == io.EOF {
		if !e.keys.ctrlC.Swap(false) {
			return "", io.EOF
		}
		// x/term ends the line it was editing on Ctrl-C and keeps its
		// text: a new editor starts an empty one. Straight to the screen:
		// the old editor would draw its line again below it.
		e.out.Write([]byte("^C\r\n"))
		next := e.newTerm()
		e.mu.Lock()
		e.term = next
		e.mu.Unlock()
		return "", ErrInterrupt
	}
	if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
		return "", err
	}
	return line, nil
}

// History returns the lines in the history, the oldest first.
func (e *Editor) History() []string {
	return e.history.lines()
}

// tab answers Tab with the cursor at pos in line: the one name the word
// before the cursor can be completed to, or the longest start the names
// share, or, when the word is that start already, the names listed above
// the line. A Tab that completes nothing changes nothing.
func (e *Editor) tab(t *term.Terminal, line string, pos int) (string, int, bool) {
	e.mu.Lock()
	complete := e.complete
	e.mu.Unlock()
	if complete == nil {
		return line, pos, true
	}
	word, names := complete(line[:pos])
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
	t.Write([]byte(strings.Join(names, "  ") + "\n"))
	return line, pos, true
}

// history is the editor's history, x/term's History: the lines, the oldest
// first, at most limit of them. x/term adds every line it reads; Add keeps
// one only while adding is set, for the ReadLine that asked for it.
type history struct {
	path  string
	limit int

	mu     sync.Mutex
	adding bool
	all    []string
}

func (h *history) setAdding(on bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.adding = on
}

// load reads the history file, keeping its newest lines.
func (h *history) load() {
	if h.path == "" {
		return
	}
	f, err := os.Open(h.path)
	if err != nil {
		return
	}
	defer f.Close()
	in := bufio.NewScanner(f)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	for in.Scan() {
		if line := in.Text(); line != "" {
			h.all = append(h.all, line)
		}
	}
	h.trim()
}

func (h *history) trim() {
	if len(h.all) > h.limit {
		h.all = append([]string(nil), h.all[len(h.all)-h.limit:]...)
	}
}

// Add keeps entry, while adding, and rewrites the history file.
func (h *history) Add(entry string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.adding {
		return
	}
	h.all = append(h.all, entry)
	h.trim()
	if h.path != "" {
		os.WriteFile(h.path, []byte(strings.Join(h.all, "\n")+"\n"), 0o600)
	}
}

// Len returns the number of lines.
func (h *history) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.all)
}

// At returns a line, 0 the newest.
func (h *history) At(idx int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.all[len(h.all)-1-idx]
}

func (h *history) lines() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.all...)
}
