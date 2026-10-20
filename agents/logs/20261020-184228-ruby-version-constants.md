# Task: define Ruby's version constants for cameo

**Started:** 2026-10-20 18:42:28
**Ended:** 2026-10-20 19:25:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 150k input, 25k output

## Objective
Define RUBY_COPYRIGHT, RUBY_DESCRIPTION, RUBY_ENGINE, RUBY_ENGINE_VERSION,
RUBY_PATCHLEVEL, RUBY_PLATFORM, RUBY_RELEASE_DATE, RUBY_REVISION and
RUBY_VERSION for every program, describing cameo and the Ruby it follows, and
refuse their reassignment.

## Progress
- [x] src/buildinfo: version, commit, date and platform of the binary, with tests
- [x] src/stdlib/version.go defines the constants; eval refuses top-level
      assignment of a builtin constant
- [x] examples/ruby_version.cameo
- [x] benchmark vm_gc_wb_ary moved from unsupported/ruby-version to
      unsupported/vm-introspection: it calls GC.start next
- [x] doc/cameo-vs-ruby.md, design document, gh_pages/ruby.html, benchmarks README
- [x] make check; `ruby run.rb works` (849 of 852 within 60 s at 8 jobs on 4 cores; the 3 others run cleanly alone, as fast as on main)

## Obstacles
- **Issue:** `cameo.version` was never set by any build, so every binary
  reported 0.0.0-dev.
  **Resolution:** the version falls back to the module version Go records;
  Go 1.24 and later stamp a pseudo-version from git on `go build`.
- **Issue:** the one benchmark held back by RUBY_VERSION calls GC.start
  right after it.
  **Resolution:** filed under unsupported/vm-introspection, not works/.

## Outcome
The nine constants are defined and documented; a pull request carries them.
