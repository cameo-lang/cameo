# Task: make every case in benchmarks/ruby/works run on cameo again

**Started:** 2026-11-11 20:25:00
**Ended:** 2026-11-11 22:10:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 400k input, 40k output

## Objective
246 of the 852 cases in `benchmarks/ruby/works/` failed on main (7a0bcb8)
and on the `ir/` stack before any timing. They mutate in a way cameo
refuses since the cases were classified: a bang method of Ruby's core
(refused by the checker), a block that assigns to a variable of the scope
around it (refused by the checker), or a mutator called without assigning
its result back (a CameoError at run time). Rewrite a case whose setup
alone mutates, move a case whose timed script mutates to
`unsupported/mutation/`, and make the manifest and the READMEs match.

## Progress
- [x] Listed the failures with `run.rb`, then each case's whole stderr: the
      checker reports bang methods before the run, so most `set--` cases
      showed `map!` in the shared prelude, not the rebinding rule
- [x] Rewrote the setup of 183 cases: the `set--` prelude
      (`x = x.map(&:to_i)` for `map!`), a hash filled in a block
      (`each_with_object`), `hash_first`'s deletes (`except`), and
      `array = array.push(x)` in `app_lc_fizzbuzz`
- [x] Moved 61 cases whose timed script mutates to `unsupported/mutation/`,
      with the manifest's reason
- [x] Checked the rewrites: each case's value, cameo against CRuby 3.4
      (warmruby), and each rewritten build, old against new on CRuby 3.3
- [x] READMEs, doc/cameo-vs-ruby.md, the conformance README, aot.md, the
      hetzner README and skill: 791 cases; the stale rows of results.tsv
      named in the compare README
- [x] hotspots.rb no longer names the moved hash_small8 and hash_small4
- [x] make check; make conformance under a UTF-8 locale; commit, pull request

## Obstacles
- **Issue:** The 246 were not all the rebinding message: 199 `set--` cases
  stopped on `array1.map!` in their prelude, 12 string cases on
  `gsub!`/`sub!`/`scrub!`, and `app_lc_fizzbuzz` and the `hash_first`
  cases on a mutator called without assigning back.
  **Resolution:** Each case was judged by what the checker and the run
  report after its setup is fixed.
- **Issue:** `time_strptime--time_strptime_4` and `_7` fail on strptime
  (`%s.%N`, and a zone name for `%z`), not on mutation.
  **Resolution:** Left in `works/`: cameo-lang/cameo#366 adds both. It
  was an open pull request on another stack then, cameo-lang/cameo#321.
- **Issue:** `ruby run.rb works` from the repository root found no files
  and exited 0.
  **Resolution:** Ran it with the full path; noted as a follow-up.
- **Issue:** warmruby crashes on `set--divide_2arity_1000` (a known
  failure, listed in the compare README).
  **Resolution:** Checked that case against CRuby 3.3, which runs it: it
  uses no syntax newer than 3.3.

## Outcome
`works/` holds 791 cases. On the `ir/` stack 789 run cleanly, and on main
(7a0bcb8) the same 789; the other two are the strptime cases that
cameo-lang/cameo#366 fixes. 61 cases moved to `unsupported/mutation/`.
`make check` passed (16 Go packages, 60 integration tests);
`make conformance` passed with LANG=C.UTF-8 (791 benchmark scripts, 58 with
a compiled method, 0 differ). Without a UTF-8 locale it fails to read the
site's samples, which cameo-lang/cameo#320 fixes. results.tsv was not
rerun: the compare README names its stale rows.

`app_lc_fizzbuzz` takes 130 s on the `ir/` stack and 70 s on main, run
alone on a 4-core container: a slowdown of the stack, not of this change.

The change was rebased onto cameo-lang/cameo#363, whose stack removes
`cameo build`, `doc/design/aot.md` and `tests/conformance/`, and lost its
count edits to those two files there. It has since moved to the bottom
of the stack, below the removal (cameo-lang/cameo#409 to
cameo-lang/cameo#363), and leaves the counts in the two files as they
are: the files go a few pull requests later.

## Insights
The checker stops at the first group of errors it finds, so a case that
fails on a bang method may also hold a block that rebinds; fix the first
and run it again before filing it.
