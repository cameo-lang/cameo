package tests

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/readline"
)

// TestReadlineEditorReadsLinesAndKeepsAsked proves ReadLine returns each
// line typed, keeps in the history only those it was asked to, and returns
// io.EOF at the end of the stream and ErrInterrupt at Ctrl-C.
func TestReadlineEditorReadsLinesAndKeepsAsked(t *testing.T) {
	st := newStream()
	e := readline.New(st)
	type result struct {
		line string
		err  error
	}
	read := make(chan result)
	next := func(add bool) result {
		go func() {
			line, err := e.ReadLine("> ", add)
			read <- result{line, err}
		}()
		return <-read
	}
	go st.typing.Write([]byte("one\r"))
	assert.Equals(t, next(true).line, "one")
	go st.typing.Write([]byte("two\r"))
	assert.Equals(t, next(false).line, "two")
	go st.typing.Write([]byte("thr\x03"))
	assert.True(t, errors.Is(next(true).err, readline.ErrInterrupt))
	go st.typing.Write([]byte("four\r"))
	assert.Equals(t, next(true).line, "four")
	assert.Equals(t, e.History(), []string{"one", "four"})
}

// TestReadlineHistoryKeepsTheNewestLinesInItsFile proves a history file
// keeps at most the limit's newest lines, and a new editor reads them back,
// so the up arrow recalls a line of the last run.
func TestReadlineHistoryKeepsTheNewestLinesInItsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	assert.NoError(t, os.WriteFile(path, []byte("a\nb\nc\n"), 0o600))
	st := newStream()
	e := readline.New(st, readline.HistoryFile(path), readline.HistoryLimit(3))
	assert.Equals(t, e.History(), []string{"a", "b", "c"})
	go st.typing.Write([]byte("d\r"))
	line, err := e.ReadLine("> ", true)
	assert.NoError(t, err)
	assert.Equals(t, line, "d")
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equals(t, string(data), "b\nc\nd\n")

	st2 := newStream()
	again := readline.New(st2, readline.HistoryFile(path), readline.HistoryLimit(2))
	assert.Equals(t, again.History(), []string{"c", "d"})
	go st2.typing.Write([]byte("\x1b[A\r"))
	line, err = again.ReadLine("> ", false)
	assert.NoError(t, err)
	assert.Equals(t, line, "d")
}

// TestReadlineScriptReadsFromTheTerminalItsHostGave proves
// Cameo::Kernel::Readline.readline reads a line from the editor the host
// gave the program, with the prompt, keeps it in the history when asked,
// and returns nil at the end of the input.
func TestReadlineScriptReadsFromTheTerminalItsHostGave(t *testing.T) {
	st := newStream()
	src := `while line = Cameo::Kernel::Readline.readline("name? ", true)
  puts "hi #{line}"
end
p Cameo::Kernel::Readline.history
`
	var out bytes.Buffer
	prog, err := cameo.Load("t.cameo", src, &out, &out)
	assert.NoError(t, err)
	prog.Terminal(readline.New(st))
	go func() {
		st.typing.Write([]byte("ann\r"))
		st.typing.Write([]byte("bo\r"))
		st.typing.Close()
	}()
	assert.NoError(t, prog.Run())
	assert.Equals(t, out.String(), "hi ann\nhi bo\n[\"ann\", \"bo\"]\n")
	st.shows(t, "name? ")
}

// TestReadlineScriptWithoutATerminalReadsNil proves readline returns nil
// and history is empty where the host gave the program no terminal.
func TestReadlineScriptWithoutATerminalReadsNil(t *testing.T) {
	var out strings.Builder
	err := cameo.Run("t.cameo", "p Cameo::Kernel::Readline.readline(\"> \"), Cameo::Kernel::Readline.history\n", &out, &out)
	assert.NoError(t, err)
	assert.Equals(t, out.String(), "nil\n[]\n")
}
