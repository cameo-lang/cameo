# Task: Site prose review and syntax highlighting

**Started:** 2026-10-19 07:25:27
**Ended:** 2026-10-19 07:50:00
**Strategy:** Refactoring (the prose) and Feature (TDD) (the highlighting)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable
**Token usage (Estimated):** -

## Objective
Reread the four pages of doc/gh_pages as a newcomer, tighten the prose
and correct what is out of date against main (module path, v0.2.2, field
numbers from #100), and highlight the site's code samples (cameo/Ruby, Go,
shell, protobuf, diff) with a vendored highlighter that needs no build
step, in a scheme that reads in light and dark mode.

## Progress
- [x] Read the pages, the code and doc/cameo-vs-ruby.md; checked the claims against src/ (File's methods, the regexp checker, enum numbering, cross-compilation for the six listed targets, the startup time)
- [x] Ran every Go sample in a scratch module that replaces the module with the checkout, the native-class one with code buf generated from the page's .proto, and the Money example in a worktree with the page's diff applied; the protobuf sample's output on go.html was stale after #100 and is regenerated
- [x] Prose edits on the four pages
- [x] Test first: TestSiteBlocksNameAHighlightedLanguage, red until the pages loaded site.js
- [x] Vendored highlight.js 11.12.0 (es/core and ruby, go, bash, protobuf, diff) from the npm tarball, site.js, theme on the site's variables
- [x] Rendered the pages in Chromium over HTTP, light and dark: every sample block highlighted, no output block, no console errors
- [x] make check

## Obstacles
- **Issue:** cdn.jsdelivr.net, cdnjs and unpkg are denied by the proxy.
  **Resolution:** registry.npmjs.org is open; the files come from the
  `@highlightjs/cdn-assets` tarball.
- **Issue:** `protoc` is not installed and GitHub releases are denied.
  **Resolution:** `go install github.com/bufbuild/buf/cmd/buf@latest` and
  `buf generate` with a local protoc-gen-go produced the sample's geopb.
- **Issue:** `ruby -run -e httpd` fails: Ruby 3.3 no longer bundles webrick.
  **Resolution:** `python3 -m http.server` served the pages for Chromium.
- **Issue:** The ark links on native.html (the repository and a file
  `coderuby/amber.go`) return 404 through raw.githubusercontent.com and
  github.com is denied, so neither could be verified. **Resolution:** the
  file link is dropped; the repository link stays, unverified.
- **Issue:** The site said 60 to 75 µs for a five-line script; a loop of
  1000 cameo.Run calls measured 17 to 19 µs. **Resolution:** the page says
  about 20 µs.

## Outcome
Four pages reread and corrected (v0.2.2, field numbers from #100, the
startup time, File's methods, a dead file link), highlighted by a vendored
highlight.js in a theme on the site's own colour variables, and a test that
holds every block to a highlighted language. The checker's output and every
sample on the site were rerun.

## Insights
Module scripts do not run over file://; the pages highlight when served
over HTTP, which GitHub Pages does, and read as before without it.
