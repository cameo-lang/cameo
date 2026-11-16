package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/syntax"
)

// codeLink is a link to the page of a code, as COMPATIBILITY.md and the
// site's compatibility.html write it.
var codeLink = regexp.MustCompile(`compatibility/([a-z0-9]+)\.html`)

// TestEveryCodeHasAPage holds every code the checker refuses with, and
// every code COMPATIBILITY.md lists, to a page of the site, which the
// refusal's message links.
func TestEveryCodeHasAPage(t *testing.T) {
	md, err := os.ReadFile("../COMPATIBILITY.md")
	assert.NoError(t, err)
	codes := slices.Clone(syntax.Codes)
	for _, m := range codeLink.FindAllStringSubmatch(string(md), -1) {
		codes = append(codes, m[1])
	}
	slices.Sort(codes)
	for _, code := range slices.Compact(codes) {
		_, err := os.Stat("../doc/gh_pages/compatibility/" + code + ".html")
		assert.NoError(t, err, "the code %s has no page", code)
		assert.Equals(t, syntax.CodePage(code), "https://cameo-lang.github.io/cameo/compatibility/"+code+".html")
	}
}

// TestEveryCodePageIsReachedAndShowsBothSides holds each page under
// compatibility/ to being linked from compatibility.html or named by the
// checker, to linking back, and to showing a sample cameo refuses and
// one it runs; TestSiteSamplesPrintTheirOutput runs both.
func TestEveryCodePageIsReachedAndShowsBothSides(t *testing.T) {
	index, err := os.ReadFile("../doc/gh_pages/compatibility.html")
	assert.NoError(t, err)
	pages, err := filepath.Glob("../doc/gh_pages/compatibility/*.html")
	assert.NoError(t, err)
	assert.True(t, len(pages) >= len(syntax.Codes), "found %d code pages", len(pages))
	for _, page := range pages {
		code := strings.TrimSuffix(filepath.Base(page), ".html")
		src, err := os.ReadFile(page)
		assert.NoError(t, err)
		linked := strings.Contains(string(index), `href="compatibility/`+code+`.html"`)
		assert.True(t, linked || slices.Contains(syntax.Codes, code), "no page links %s", sitePageName(page))
		assert.True(t, strings.Contains(string(src), `<h1>`+code+` — `), "%s: the title does not name the code", code)
		assert.True(t, strings.Contains(string(src), `href="../compatibility.html#not-expected-to-work"`), "%s: no link back", code)
		assert.True(t, strings.Contains(string(src), `<pre class="cameo rejected">`), "%s: no sample cameo refuses", code)
		assert.True(t, strings.Contains(string(src), `<pre class="cameo">`), "%s: no sample cameo runs", code)
		for _, id := range []string{"refused", "why", "instead"} {
			assert.True(t, strings.Contains(string(src), `<h2 id="`+id+`">`), "%s: no section %s", code, id)
		}
	}
}
