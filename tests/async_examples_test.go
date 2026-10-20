package tests

import (
	"bytes"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// asyncExamples are the scripts of examples/async, which run unchanged on
// cameo and, with ruby/lib/cameo/runner.rb, on CRuby.
func asyncExamples(t *testing.T) []string {
	t.Helper()
	scripts, err := filepath.Glob("../examples/async/*.rb")
	assert.NoError(t, err)
	assert.True(t, len(scripts) > 0, "no scripts in examples/async")
	return scripts
}

func expectedOf(t *testing.T, script string) string {
	t.Helper()
	want, err := os.ReadFile(strings.TrimSuffix(script, ".rb") + ".expected")
	assert.NoError(t, err)
	return string(want)
}

// TestAsyncExamplesOnCameo runs each async example on cameo and holds it to
// its expected output.
func TestAsyncExamplesOnCameo(t *testing.T) {
	for _, script := range asyncExamples(t) {
		t.Run(filepath.Base(script), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.NoError(t, cameo.RunFile(script, &stdout, &stderr))
			assert.Equals(t, stderr.String(), "")
			assert.Equals(t, stdout.String(), expectedOf(t, script))
		})
	}
}

// TestAsyncExamplesOnRuby runs each async example on CRuby with
// Cameo::Runner from ruby/lib and holds it to the output cameo prints. It
// is skipped where no ruby is on PATH.
func TestAsyncExamplesOnRuby(t *testing.T) {
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("no ruby on PATH")
	}
	for _, script := range asyncExamples(t) {
		t.Run(filepath.Base(script), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(ruby, "-I", "../ruby/lib", script)
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			assert.NoError(t, cmd.Run(), "%s", stderr.String())
			assert.Equals(t, stderr.String(), "")
			assert.Equals(t, stdout.String(), expectedOf(t, script))
		})
	}
}

// TestSiteShowsTheAsyncExamples holds the async guide's samples to
// examples/async: each script's code, after its leading comment, and its
// expected output, in order.
func TestSiteShowsTheAsyncExamples(t *testing.T) {
	page, err := os.ReadFile("../doc/gh_pages/async.html")
	assert.NoError(t, err)
	// The examples are the section between the examples heading and the
	// Ruby one; the workflow sample above it has no output block.
	section := regexp.MustCompile(`(?s)<h2 id="examples">.*?<h2 id="ruby">`).FindString(string(page))
	assert.True(t, section != "", "async.html has no examples section")
	shown := regexp.MustCompile(`(?s)<pre class="cameo"><code>(.*?)</code></pre>\s*<pre class="output"><code>(.*?)</code></pre>`).FindAllStringSubmatch(section, -1)
	scripts := asyncExamples(t)
	assert.Equals(t, len(shown), len(scripts))
	// The webfetch workflow is the script of examples/webfetch/main.go.
	mainGo, err := os.ReadFile("../examples/webfetch/main.go")
	assert.NoError(t, err)
	script := regexp.MustCompile("(?s)const script = `\n(.*?)\n`\n").FindStringSubmatch(string(mainGo))
	assert.NotNil(t, script, "main.go has no script constant")
	assert.True(t, strings.Contains(html.UnescapeString(string(page)), script[1]), "async.html does not show the webfetch script")
	for i, script := range scripts {
		src, err := os.ReadFile(script)
		assert.NoError(t, err)
		lines := strings.Split(strings.TrimRight(string(src), "\n"), "\n")
		for len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
			lines = lines[1:]
		}
		code := strings.Trim(strings.Join(lines, "\n"), "\n")
		assert.Equals(t, html.UnescapeString(shown[i][1]), code)
		assert.Equals(t, html.UnescapeString(shown[i][2]), strings.TrimRight(expectedOf(t, script), "\n"))
	}
}
