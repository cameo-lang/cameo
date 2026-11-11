# Task: make the conformance run and the CRuby benchmarks pass on main

**Started:** 2026-11-11 11:07:35
**Ended:** 2026-11-11 11:25:14
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 250k input, 30k output

## Objective
`ruby tests/conformance/run.rb` (`make conformance`) failed on main for
two reasons. In a locale that is not UTF-8 it stopped reading the site's
pages with `invalid byte sequence in US-ASCII`. In a UTF-8 locale 241
scripts under `benchmarks/ruby/works` failed to build. Done means the
conformance run exits 0 in a POSIX locale, `ruby benchmarks/ruby/run.rb`
passes and `make check` is green.

## Progress
- [x] Reproduced both failures on main under `LC_CTYPE=POSIX` and `LC_ALL=C.UTF-8`
- [x] `tests/conformance/run.rb` reads the pages and the output of `go build` and `cameo build` as UTF-8
- [x] Sorted the 241 build failures: 30 blocks rebinding a variable of the scope around them, 211 calls to one of Ruby's bang methods (`map!` in 199 `set--` preludes, `gsub!`, `sub!`, `scrub!`)
- [x] Found that `ruby benchmarks/ruby/run.rb works` failed on main too: 246 files, the 241 plus five that fail when run
- [x] Rewrote the cases whose refused idiom only builds data (161 `set--` preludes, 25 scripts that fill a hash in a block, `kernel_tap`, `app_lc_fizzbuzz`, `hash_first`)
- [x] Moved the cases that time the mutation itself to `unsupported/mutation` (54) and the two `Time.strptime` defects to `bugs/`; MANIFEST.tsv and README.md follow
- [x] Checked each rewrite against CRuby: original and rewrite under CRuby 3.3, rewrite under cameo, the same summary of the data built
- [x] Conformance run exits 0 under `LC_CTYPE=POSIX`
- [x] `ruby benchmarks/ruby/run.rb` exits 0 (1318 files, every one under `works/` clean); `make check` green

## Obstacles
- **Issue:** The task named 241 block-rebinding failures; 211 of them were
  the refusal of Ruby's bang methods, a separate checker rule.
  **Resolution:** Handled both by the same test: rewrite when the refused
  idiom is incidental, move when it is what the benchmark times.
- **Issue:** `benchmarks/ruby/run.rb works` was not green on main either,
  so "still passes" had no baseline. The conformance run only runs the
  scripts that compile a method, so five runtime failures under `works/`
  did not show there.
  **Resolution:** Classified those five as well.
- **Issue:** `ruby run.rb works` resolves `works` against the current
  directory, so run from the repository root it found 0 files and exited 0.
  **Resolution:** Ran it from `benchmarks/ruby`, as its README shows.
- **Issue:** The `set--` cases cannot run under CRuby 3.3 (they use `it`,
  from Ruby 3.4), so the CRuby comparison covered their prelude alone.
  **Resolution:** Compared the prelude's arrays and sets; the timed bodies
  are unchanged.

## Outcome
The conformance run exits 0 in a POSIX locale: 895 scripts, 68 with
compiled methods, all printing the same. `works/` holds 796 scripts, down
from 852: 161 `set--` preludes, 25 scripts that fill a hash in a block, `kernel_tap`,
`app_lc_fizzbuzz` and the two `hash_first` cases are rewritten; 54 cases
that time a mutation moved to `unsupported/mutation`; two `Time.strptime`
cases moved to `bugs/`. A separate task is suggested for the strptime
defect.

## Insights
The rewrite of every benchmark script for "blocks capture values"
(20261105-120000-closures-and-attributes.md) did not survive the replay
of the history: no file under `works/` had been touched since. Neither the
conformance run nor `benchmarks/ruby/run.rb` runs in CI, so nothing
noticed. A rule the checker adds should be followed by both runs.
