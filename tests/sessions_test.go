package tests

import (
	"bufio"
	"bytes"
	"errors"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// builtCameo is the cameo command the tests of this package built, in a
// directory TestMain removes.
var builtCameo struct {
	once sync.Once
	dir  string
	out  []byte
	err  error
}

// buildCameo builds the cameo command, once for all tests, and returns
// its path.
func buildCameo(t *testing.T) string {
	t.Helper()
	b := &builtCameo
	b.once.Do(func() {
		if b.dir, b.err = os.MkdirTemp("", "cameo-tests-"); b.err != nil {
			return
		}
		b.out, b.err = exec.Command("go", "build", "-o", filepath.Join(b.dir, "cameo"), "../src/cmd/cameo").CombinedOutput()
	})
	assert.NoError(t, b.err, "go build: %s", b.out)
	return filepath.Join(b.dir, "cameo")
}

// dumpPath is the name of the file a paused script's state is written to.
var dumpPath = regexp.MustCompile(`\S*cameo-\S+\.dump`)

// sessionMatches is true when the lines of got are those of want, a line
// "..." of want standing for any lines, none included.
func sessionMatches(want, got []string) bool {
	if len(want) == 0 {
		return len(got) == 0
	}
	if want[0] == "..." {
		for skip := 0; skip <= len(got); skip++ {
			if sessionMatches(want[1:], got[skip:]) {
				return true
			}
		}
		return false
	}
	return len(got) > 0 && want[0] == got[0] && sessionMatches(want[1:], got[1:])
}

// sessionLines splits what a command printed into lines, every dump's
// path replaced by DUMP: it differs from run to run.
func sessionLines(s string) []string {
	return strings.Split(strings.TrimSuffix(dumpPath.ReplaceAllString(s, "DUMP"), "\n"), "\n")
}

// session runs the commands a document shows in one directory, as its
// reader would, and remembers the dump a paused script left.
type session struct {
	t    *testing.T
	bin  string
	dir  string
	dump string
}

// run runs one command of a document and holds what it prints to want.
// "cameo" is the built command, and "go run ./examples/x" runs at the
// root of the repository. A line "^C" in want is an interrupt: it is sent
// when the script has printed the line before it, and what follows is
// what cameo says on standard error as it pauses.
func (s *session) run(command, want string) {
	s.t.Helper()
	args := strings.Fields(command)
	var cmd *exec.Cmd
	switch {
	case args[0] == "cameo":
		if s.dump != "" {
			for i, a := range args {
				args[i] = dumpPath.ReplaceAllString(a, s.dump)
			}
		}
		cmd = exec.Command(s.bin, args[1:]...)
		cmd.Dir = s.dir
		// a paused script's dump goes into the session's directory
		cmd.Env = append(os.Environ(), "TMPDIR="+s.dir)
	case strings.HasPrefix(command, "go run ./examples/"):
		cmd = exec.Command("go", args[1:]...)
		cmd.Dir = ".."
	default:
		s.t.Fatalf("a session cannot run %q", command)
	}
	before, after, interrupted := strings.Cut(want, "\n^C\n")
	if !interrupted {
		out, err := cmd.CombinedOutput()
		assert.NoError(s.t, err, "%s: %s", command, out)
		if want != "" {
			assert.True(s.t, sessionMatches(sessionLines(want), sessionLines(string(out))),
				"%s printed\n%s\nand the document shows\n%s", command, out, want)
		}
		return
	}

	wantOut := sessionLines(before)
	trigger := wantOut[len(wantOut)-1]
	stdout, err := cmd.StdoutPipe()
	assert.NoError(s.t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	assert.NoError(s.t, cmd.Start())
	var printed []string
	lines := bufio.NewScanner(stdout)
	for lines.Scan() {
		printed = append(printed, lines.Text())
		if lines.Text() == trigger {
			assert.NoError(s.t, cmd.Process.Signal(os.Interrupt))
		}
	}
	var exit *exec.ExitError
	assert.True(s.t, errors.As(cmd.Wait(), &exit), "%s ended without an error", command)
	assert.Equals(s.t, exit.ExitCode(), 3)
	assert.True(s.t, sessionMatches(wantOut, printed),
		"%s printed\n%s\nand the document shows\n%s", command, strings.Join(printed, "\n"), before)
	assert.True(s.t, sessionMatches(sessionLines(after), sessionLines(stderr.String())),
		"%s said\n%s\nand the document shows\n%s", command, stderr.String(), after)
	s.dump = dumpPath.FindString(stderr.String())
	assert.True(s.t, s.dump != "", "no dump named in %s", stderr.String())
}

// siteNamedFile is a block of the site that shows a file a session of the
// same page uses: its data-name is the file's name.
var siteNamedFile = regexp.MustCompile(`(?s)<pre data-name="([^"]+)" class="[^"]*"><code>(.*?)</code></pre>`)

// siteSession is a command on the site with what it prints under it.
var siteSession = regexp.MustCompile(`(?s)<pre class="sh"><code>((?:cameo|go run \./examples/)[^\n<]*)</code></pre>\s*<pre class="output"><code>(.*?)</code></pre>`)

// TestSiteSessionsPrintWhatThePagesShow runs the shell sessions of the
// site: each page's named files are written into a directory, and every
// command of one line with an output block under it is run there, in the
// page's order. So ^C, cameo resume and cameo --io=file do what the pages
// say, and an example prints what its page shows.
func TestSiteSessionsPrintWhatThePagesShow(t *testing.T) {
	bin := buildCameo(t)
	pages, err := filepath.Glob("../doc/gh_pages/*.html")
	assert.NoError(t, err)
	ran := 0
	for _, page := range pages {
		src, err := os.ReadFile(page)
		assert.NoError(t, err)
		commands := siteSession.FindAllStringSubmatch(string(src), -1)
		if len(commands) == 0 {
			continue
		}
		ran += len(commands)
		t.Run(filepath.Base(page), func(t *testing.T) {
			s := &session{t: t, bin: bin, dir: t.TempDir()}
			for _, m := range siteNamedFile.FindAllStringSubmatch(string(src), -1) {
				assert.NoError(t, os.WriteFile(filepath.Join(s.dir, m[1]), []byte(html.UnescapeString(m[2])+"\n"), 0o644))
			}
			for _, m := range commands {
				s.run(html.UnescapeString(m[1]), html.UnescapeString(m[2]))
			}
		})
	}
	assert.True(t, ran >= 6, "ran %d commands of the site", ran)
}

// readmeBlock returns the fenced block of the README that starts with
// the line first, without its fences.
func readmeBlock(t *testing.T, readme, first string) string {
	t.Helper()
	_, rest, found := strings.Cut(readme, "```\n"+first+"\n")
	assert.True(t, found, "the README has no block that starts with %q", first)
	body, _, _ := strings.Cut(rest, "```")
	return body
}

// irbPrompt is a line of an irb session: the prompt and what was typed.
var irbPrompt = regexp.MustCompile(`^(cameo\(main\):\d+> )(.*)$`)

// TestReadmeIRBSessionIsWhatIRBPrints types the lines of the README's irb
// session into cameo irb and holds its answers to the README's.
func TestReadmeIRBSessionIsWhatIRBPrints(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	assert.NoError(t, err)
	var typed, want strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(readmeBlock(t, string(readme), "$ cameo irb"), "\n"), "\n") {
		if m := irbPrompt.FindStringSubmatch(line); m != nil {
			typed.WriteString(m[2] + "\n")
			want.WriteString(m[1]) // a pipe echoes nothing, so the answer follows the prompt
			continue
		}
		want.WriteString(line + "\n")
	}
	cmd := exec.Command(buildCameo(t), "irb")
	cmd.Stdin = strings.NewReader(typed.String())
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "%s", out)
	// irb prompts once more and ends at the end of its input
	got := string(out)
	if i := strings.LastIndex(got, "\ncameo(main):"); i >= 0 {
		got = got[:i+1]
	}
	assert.Equals(t, got, want.String())
}

// TestReadmePauseSessionIsWhatCameoPrints runs the README's ^C session on
// count.cameo, the script of the site's page on pausing.
func TestReadmePauseSessionIsWhatCameoPrints(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	assert.NoError(t, err)
	page, err := os.ReadFile("../doc/gh_pages/pause.html")
	assert.NoError(t, err)
	s := &session{t: t, bin: buildCameo(t), dir: t.TempDir()}
	for _, m := range siteNamedFile.FindAllStringSubmatch(string(page), -1) {
		assert.NoError(t, os.WriteFile(filepath.Join(s.dir, m[1]), []byte(html.UnescapeString(m[2])+"\n"), 0o644))
	}
	comment := regexp.MustCompile(`\s+# .*$`)
	commands := strings.Split("\n"+readmeBlock(t, string(readme), "$ cameo count.cameo"), "\n$ ")
	commands[0] = "cameo count.cameo" + commands[0]
	assert.True(t, len(commands) == 3, "the README's pause session has %d commands", len(commands))
	for _, c := range commands {
		command, want, _ := strings.Cut(c, "\n")
		s.run(comment.ReplaceAllString(command, ""), strings.TrimSuffix(want, "\n"))
	}
}

// goComment is a comment at the end of a line of Go.
var goComment = regexp.MustCompile(`\s*// .*$`)

// TestReadmeGoExcerptIsOfTheFirstExample holds the Go the README shows to
// examples/ex1/main.go, which is compiled and has its own test: every
// line of the excerpt is a line of the program, in the program's order. A
// line that is a comment alone stands for what the excerpt leaves out.
func TestReadmeGoExcerptIsOfTheFirstExample(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	assert.NoError(t, err)
	program, err := os.ReadFile("../examples/ex1/main.go")
	assert.NoError(t, err)
	_, rest, found := strings.Cut(string(readme), "```go\n")
	assert.True(t, found, "the README shows no Go")
	excerpt, _, _ := strings.Cut(rest, "```")
	lines := strings.Split(string(program), "\n")
	at, shown := 0, 0
	for _, line := range strings.Split(excerpt, "\n") {
		line = strings.TrimSpace(goComment.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		shown++
		for at < len(lines) && strings.TrimSpace(lines[at]) != line {
			at++
		}
		assert.True(t, at < len(lines), "examples/ex1/main.go has no line %q after the lines before it", line)
		at++
	}
	assert.True(t, shown >= 5, "the README's Go excerpt has %d lines", shown)
}
