package tests

import (
	"bytes"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"

	"github.com/cameo-lang/cameo/src/cameo"
)

// siteSample is a cameo sample on the site, and the output block that
// follows it, if any. A sample marked rejected must fail the check with
// the output block as its error.
var siteSample = regexp.MustCompile(`(?s)<pre class="cameo( rejected)?"><code>(.*?)</code></pre>\s*(?:<pre class="output(?: error)?"><code>(.*?)</code></pre>)?`)

// TestSiteSamplesPrintTheirOutput runs every cameo sample on the GitHub
// Pages site in doc/gh_pages and holds it to the output printed under it.
func TestSiteSamplesPrintTheirOutput(t *testing.T) {
	pages, err := filepath.Glob("../doc/gh_pages/*.html")
	assert.NoError(t, err)
	samples := 0
	for _, page := range pages {
		src, err := os.ReadFile(page)
		assert.NoError(t, err)
		for i, m := range siteSample.FindAllStringSubmatch(string(src), -1) {
			samples++
			rejected := m[1] != ""
			script := html.UnescapeString(m[2])
			want := html.UnescapeString(m[3])
			name := filepath.Base(page) + "/" + strconv.Itoa(i+1)
			t.Run(name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := cameo.Run("script.cameo", script, &stdout, &stderr)
				if rejected {
					assert.NotNil(t, err, "a rejected sample ran:\n%s", script)
					assert.Equals(t, err.Error(), want)
					return
				}
				assert.NoError(t, err)
				assert.Equals(t, stderr.String(), "")
				if want != "" {
					assert.Equals(t, strings.TrimSuffix(stdout.String(), "\n"), want)
				}
			})
		}
	}
	assert.True(t, samples >= 14, "found %d samples on the site", samples)
}

// TestSiteShowsTheWebFetchExample holds the Go program on the site's host
// page to examples/webfetch, which its own test runs.
func TestSiteShowsTheWebFetchExample(t *testing.T) {
	page, err := os.ReadFile("../doc/gh_pages/go.html")
	assert.NoError(t, err)
	program, err := os.ReadFile("../examples/webfetch/main.go")
	assert.NoError(t, err)
	shown := regexp.MustCompile(`(?s)<h2 id="promises">.*?<pre class="go"><code>(.*?)</code></pre>`).FindStringSubmatch(string(page))
	assert.NotNil(t, shown, "go.html has no promises section with a Go program")
	assert.Equals(t, html.UnescapeString(shown[1]), strings.TrimSuffix(string(program), "\n"))
}

// sitePre is a <pre> on the site with its classes; the first names the
// block's language, or output for what a sample printed.
var sitePre = regexp.MustCompile(`<pre class="([^"]*)">`)

// siteLanguages maps the class of a <pre> to the highlight.js language
// file site.js loads for it, as site.js does.
var siteLanguages = map[string]string{
	"cameo": "ruby.min.js",
	"ruby":  "ruby.min.js",
	"go":    "go.min.js",
	"sh":    "bash.min.js",
	"proto": "protobuf.min.js",
	"diff":  "diff.min.js",
}

// TestSiteBlocksNameAHighlightedLanguage holds every <pre> on the site to
// a class site.js highlights, or output, and every page to loading
// site.js with the language files it imports in place.
func TestSiteBlocksNameAHighlightedLanguage(t *testing.T) {
	siteJS, err := os.ReadFile("../doc/gh_pages/site.js")
	assert.NoError(t, err)
	for class, file := range siteLanguages {
		assert.True(t, strings.Contains(string(siteJS), "./hljs/"+file), "site.js does not import %s", file)
		assert.True(t, strings.Contains(string(siteJS), "  "+class+": "), "site.js does not highlight pre.%s", class)
		_, err := os.Stat("../doc/gh_pages/hljs/" + file)
		assert.NoError(t, err)
	}
	pages, err := filepath.Glob("../doc/gh_pages/*.html")
	assert.NoError(t, err)
	blocks := 0
	for _, page := range pages {
		src, err := os.ReadFile(page)
		assert.NoError(t, err)
		name := filepath.Base(page)
		assert.True(t, strings.Contains(string(src), `<script type="module" src="site.js"></script>`), "%s does not load site.js", name)
		for _, m := range sitePre.FindAllStringSubmatch(string(src), -1) {
			blocks++
			class := strings.Fields(m[1])[0]
			_, known := siteLanguages[class]
			assert.True(t, known || class == "output", "%s: <pre class=%q> names no language the site highlights", name, m[1])
		}
	}
	assert.True(t, blocks >= 30, "found %d blocks on the site", blocks)
}
