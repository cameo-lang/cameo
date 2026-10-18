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

	"github.com/radiospiel/amber/src/assert"

	"github.com/radiospiel/amber/src/amber"
)

// siteSample is an amber sample on the site, and the output block that
// follows it, if any. A sample marked rejected must fail the check with
// the output block as its error.
var siteSample = regexp.MustCompile(`(?s)<pre class="amber( rejected)?"><code>(.*?)</code></pre>\s*(?:<pre class="output(?: error)?"><code>(.*?)</code></pre>)?`)

// TestSiteSamplesPrintTheirOutput runs every amber sample on the GitHub
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
				err := amber.Run("script.arb", script, &stdout, &stderr)
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
