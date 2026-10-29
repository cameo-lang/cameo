package tests

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/irb"
)

func newIRBSession(t *testing.T, stdout io.Writer) *irb.Session {
	t.Helper()
	prog, err := cameo.Load("(irb)", "", stdout, stdout)
	assert.NoError(t, err)
	return irb.NewSession(prog)
}

// TestIRBSessionKeepsWhatALineDefined proves a session evaluates each line
// at the top level of one program, and numbers its prompts.
func TestIRBSessionKeepsWhatALineDefined(t *testing.T) {
	var printed strings.Builder
	s := newIRBSession(t, &printed)
	assert.Equals(t, s.Prompt(), "cameo(main):001> ")
	out, _ := s.Eval("x = 20")
	assert.Equals(t, out, "=> 20\n")
	out, done := s.Eval("puts x + 22")
	assert.Equals(t, out, "=> nil\n")
	assert.Equals(t, printed.String(), "42\n")
	assert.Equals(t, s.Prompt(), "cameo(main):003> ")
	assert.False(t, done)
}

// TestIRBSessionWaitsForAnIncompleteExpression proves a line that leaves an
// expression open waits for the next, and Cancel drops it.
func TestIRBSessionWaitsForAnIncompleteExpression(t *testing.T) {
	s := newIRBSession(t, io.Discard)
	out, _ := s.Eval("def twice(n)")
	assert.Equals(t, out, "")
	assert.Equals(t, s.Prompt(), "cameo(main):001* ")
	s.Eval("  n * 2")
	s.Eval("end")
	out, _ = s.Eval("twice(21)")
	assert.Equals(t, out, "=> 42\n")
	s.Eval("[1,")
	s.Cancel()
	assert.Equals(t, s.Prompt(), "cameo(main):003> ")
}

// TestIRBSessionShowsAnErrorAndEndsAtExit proves an error is the line's
// output, and exit ends the session.
func TestIRBSessionShowsAnErrorAndEndsAtExit(t *testing.T) {
	s := newIRBSession(t, io.Discard)
	out, done := s.Eval(`raise "boom"`)
	assert.True(t, strings.Contains(out, "boom"), "output %q", out)
	assert.False(t, done)
	_, done = s.Eval("exit")
	assert.True(t, done)
}

// stream is a terminal's stream for a test: the keys typed, and the screen.
type stream struct {
	keys   *io.PipeReader
	typing *io.PipeWriter
	screen chan string
}

func newStream() *stream {
	r, w := io.Pipe()
	return &stream{keys: r, typing: w, screen: make(chan string, 1024)}
}

func (s *stream) Read(p []byte) (int, error)  { return s.keys.Read(p) }
func (s *stream) Write(p []byte) (int, error) { s.screen <- string(p); return len(p), nil }

// shows waits until the screen showed want, and fails after a second.
func (s *stream) shows(t *testing.T, want string) {
	t.Helper()
	var seen strings.Builder
	deadline := time.After(time.Second)
	for !strings.Contains(seen.String(), want) {
		select {
		case p := <-s.screen:
			seen.WriteString(p)
		case <-deadline:
			t.Fatalf("the screen showed %q, not %q", seen.String(), want)
		}
	}
}

// TestIRBTerminalEditsAndRunsLines proves the terminal runs a typed line,
// brings the last one back on the up arrow, drops a line on Ctrl-C and ends
// on Ctrl-D.
func TestIRBTerminalEditsAndRunsLines(t *testing.T) {
	st := newStream()
	term := irb.NewTerminal(st)
	term.SetSize(80, 24)
	prog, err := cameo.Load("(irb)", "", term, term)
	assert.NoError(t, err)
	ended := make(chan error, 1)
	go func() { ended <- term.Run(irb.NewSession(prog)) }()

	st.shows(t, "cameo(main):001> ")
	st.typing.Write([]byte("puts 1 + 2\r"))
	st.shows(t, "3\r\n")
	st.shows(t, "=> nil\r\ncameo(main):002> ")
	st.typing.Write([]byte("\x1b[A\r"))
	st.shows(t, "=> nil\r\ncameo(main):003> ")
	st.typing.Write([]byte("def broken\x03"))
	st.shows(t, "^C\r\ncameo(main):003> ")
	st.typing.Write([]byte("40 + 2\r"))
	st.shows(t, "=> 42")
	st.typing.Write([]byte("\x04"))
	select {
	case err := <-ended:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Ctrl-D did not end the session")
	}
}

// TestIRBCompletesNamesAndMethods proves Tab's completions: the variables
// and methods of the top level, a constant, and after a dot the methods of
// a variable's value or a literal, without calling a method to find them.
func TestIRBCompletesNamesAndMethods(t *testing.T) {
	s := newIRBSession(t, io.Discard)
	s.Eval("greeting = 'hi'")
	s.Eval("def greet(n) = n")
	word, names := s.Complete("gre")
	assert.Equals(t, word, "gre")
	assert.Equals(t, names, []string{"greet", "greeting"})
	_, names = s.Complete("x = Integ")
	assert.Equals(t, names, []string{"Integer"})
	word, names = s.Complete("greeting.upc")
	assert.Equals(t, word, "upc")
	assert.Equals(t, names, []string{"upcase"})
	_, names = s.Complete("42.ev")
	assert.Equals(t, names, []string{"even?"})
	// greet is a method: completing after its dot would call it.
	_, names = s.Complete("greet.")
	assert.Equals(t, len(names), 0)
}

// TestIRBTerminalCompletesOnTab proves Tab completes the one name a word
// can be, and lists them when there are several.
func TestIRBTerminalCompletesOnTab(t *testing.T) {
	st := newStream()
	term := irb.NewTerminal(st)
	term.SetSize(80, 24)
	prog, err := cameo.Load("(irb)", "", term, term)
	assert.NoError(t, err)
	go term.Run(irb.NewSession(prog))

	st.shows(t, "cameo(main):001> ")
	st.typing.Write([]byte("alpha = 1\r"))
	st.shows(t, "cameo(main):002> ")
	st.typing.Write([]byte("alp\t\r"))
	st.shows(t, "=> 1\r\ncameo(main):003> ")
	st.typing.Write([]byte("alpine = 2\r"))
	st.shows(t, "cameo(main):004> ")
	st.typing.Write([]byte("alp\t"))
	st.shows(t, "alpha  alpine")
}

// TestIRBTerminalKeepsTheKeysAfterACtrlC proves the keys that arrive with a
// Ctrl-C, in one read, reach the line after it.
func TestIRBTerminalKeepsTheKeysAfterACtrlC(t *testing.T) {
	st := newStream()
	term := irb.NewTerminal(st)
	prog, err := cameo.Load("(irb)", "", term, term)
	assert.NoError(t, err)
	go term.Run(irb.NewSession(prog))

	st.shows(t, "cameo(main):001> ")
	st.typing.Write([]byte("drop me\x036 * 7\r"))
	st.shows(t, "=> 42")
}
