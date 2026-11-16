package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/codes"
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
	all := slices.Clone(syntax.Codes)
	for _, m := range codeLink.FindAllStringSubmatch(string(md), -1) {
		all = append(all, m[1])
	}
	slices.Sort(all)
	for _, code := range slices.Compact(all) {
		_, err := os.Stat("../doc/gh_pages/compatibility/" + code + ".html")
		assert.NoError(t, err, "the code %s has no page", code)
		assert.Equals(t, codes.Page(code), "https://cameo-lang.github.io/cameo/compatibility/"+code+".html")
	}
}

// TestRuntimeRefusalsLinkThePageOfTheirCode proves that what cameo leaves
// out, met when the script runs, ends its message with the link to the
// page of its code, as the checker's refusals do, and that the page is
// there.
func TestRuntimeRefusalsLinkThePageOfTheirCode(t *testing.T) {
	cases := []struct{ code, script string }{
		{"mut", "a = [1]\na << 2"},
		{"mut", "a = [1]\na.send(:sort!)"},
		{"meta", "class A\nend\nA.new.send(:initialize)"},
		{"meta", "p 1.singleton_class"},
		{"lit", "Range.new(1, 2)"},
		{"bobj", "Class.new(BasicObject)"},
		{"req", `require "socket"`},
		{"thr", "Thread.new { 1 }"},
		{"proc", `p ENV["HOME"]`},
		{"struct", "Struct.new(:x)"},
		{"data", "Data.define(:x)"},
		{"rat", "Rational(1, 3)"},
		{"id", `"x".equal?("x")`},
		{"next", "[1].each.next"},
		{"r2k", "ruby2_keywords"},
		{"vis", "p 1.private_methods"},
		{"bin", `b"\xff".upcase`},
		{"bin", `"é".getbyte(0)`},
		{"inv", `b"\xff".force_encoding("UTF-8")`},
		{"enc", `"a".encode("UTF-16LE")`},
		{"mar", `Marshal.load("\x04\bi\x06".b)`},
		{"io", `File.read("x")`},
		{"re", `Regexp.new("(?<=a)b")`},
		{"bin", `"v=#{b"\xff"}"`},
		{"meta", "class A\nend\nA.allocate"},
		{"lib", "require \"zlib\"\nZlib::Deflate.new"},
		{"lib", "require \"uri\"\nURI.regexp"},
		{"io", "require \"digest\"\nDigest::SHA256.file(\"x\")"},
	}
	for _, c := range cases {
		t.Run(c.code+"/"+c.script, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cameo.Run("t.cameo", c.script, &stdout, &stderr)
			assert.NotNil(t, err, "%q ran", c.script)
			assert.True(t, strings.Contains(err.Error(), " (see "+codes.URL(c.code)+") ("), "%q: %v", c.script, err)
			_, statErr := os.Stat("../doc/gh_pages/compatibility/" + c.code + ".html")
			assert.NoError(t, statErr, "the code %s has no page", c.code)
		})
	}
	// An error that is a bug or the script's own links nothing: a call of
	// a method nobody defined, and to_c on a class of the script's own,
	// which lacks its own method rather than cameo's Complex.
	for _, script := range []string{"nope(1)", "class Money\nend\nMoney.new.to_c"} {
		var stdout, stderr bytes.Buffer
		err := cameo.Run("t.cameo", script, &stdout, &stderr)
		assert.True(t, err != nil && !strings.Contains(err.Error(), "(see "), "%q: %v", script, err)
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
