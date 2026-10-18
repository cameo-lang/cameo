# Task: GitHub Pages site for amber

**Started:** 2026-10-18 10:44:53
**Ended:** 2026-10-18 10:59:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 250k input, 60k output

## Objective
A static site under `doc/gh_pages` that presents amber to Ruby and Go
programmers, deployed by a Pages workflow.

## Progress
- [x] Read the sources: README, amber-vs-ruby, the design, the host API, ark's Spicy.tools
- [x] Samples, each run: amber with bin/amber, Ruby with ruby 3.3.6, Go in a
      scratch module that requires amber v0.1.1
- [x] Pages, stylesheet, workflow
- [x] tests/site_test.go runs every amber sample on the site against its output
- [x] doc/amber-vs-ruby.md corrected where the binary disagreed
- [x] make check, pull request

## Obstacles
- **Issue:** `go get github.com/radiospiel/amber@v0.1.1` alone leaves the
  build failing with "missing go.sum entry" for amber's dependencies.
  **Resolution:** the site says to run `go mod tidy` after it.
- **Issue:** `Call` panics when a script returns a Hash with Symbol keys:
  `ToGo` makes the key an `object.Value`, which a Go map cannot hash.
  **Resolution:** unresolved; the site states it and the workaround. What a
  Symbol key becomes in Go is a decision for a separate change.
- **Issue:** `in Float | Integer => r` binds `r` to nil; Ruby binds the
  matched value. **Resolution:** unresolved; the shared sample avoids it.
- **Issue:** doc/amber-vs-ruby.md listed `system`, `exec` and `spawn` as
  rejected by the checker; they raise NoMethodError at run time.
  `Proc#curry` raises NotImplementedError and `Integer()` with a base
  ArgumentError, not NameError or NoMethodError. **Resolution:** fixed.
- **Issue:** a native `to_s` on a builtin class is not what `puts` prints.
  **Resolution:** the site's stdlib example adds the class to
  src/eval/inspect.go, as Pathname is; verified in a worktree of v0.1.1.
- **Issue:** headless Chromium renders at least 500px wide, so a 390px
  screenshot clips. **Resolution:** checked the layout at 500px.

## Outcome
Four pages and a stylesheet in doc/gh_pages, a Pages workflow, and a test
that keeps the amber samples true. The host API cannot define classes;
the site says so. GitHub Pages must be switched on with source "GitHub
Actions".

## Insights
Generating the HTML's Go samples and outputs from the files that were run
kept transcription errors out.
