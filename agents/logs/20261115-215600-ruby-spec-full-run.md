# Task: run all of ruby/spec, and write COMPATIBILITY.md from the run

**Started:** 2026-11-15 21:56:00
**Ended:** 2026-11-15 22:40:00
**Strategy:** Bug Fix (the first three pull requests), Feature (TDD) (the fourth)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Claude
**Token usage (Estimated):** 400k input, 60k output

## Objective
`make spec-status` ran `language/` and `core/` alone, stopped in the
POSIX locale, and wrote status.tsv only. It should run every directory of
ruby/spec at the commit specs/ was sorted at, and write COMPATIBILITY.md,
the counts per category with the reasons and the bugs, from status.tsv.

## Progress
- [x] run.rb reads what it captures as UTF-8 in any locale
- [x] the rewrite keeps the closing parenthesis of `m(3, &b)` under Prism 0.19
- [x] classify.rb knows every refusal of the String rules
- [x] run.rb runs all six directories at the commit COUNTS.md names, and
  names the stage of a file that did not run
- [x] an example CRuby does not pass, or does not run, is the harness's
- [x] classify.rb knows the parts of libraries stdlib.md leaves out
- [x] compatibility.rb writes COMPATIBILITY.md; make spec-status runs both
- [x] specs/test/ and make spec-tests for the three scripts
- [x] the documents that quote the counts

## Obstacles
- **Issue:** run.rb stopped in this container: `strip` on the checker's
  output, read as US-ASCII.
  **Resolution:** every captured output is read as UTF-8; a test runs
  run.rb with `LC_ALL=C`.
- **Issue:** four files stopped on a parse error the rewrite made.
  Prism 0.19 (Ruby 3.3) ends a call such as `m(3, &b)` before its
  closing parenthesis.
  **Resolution:** the rewriter takes the end from the closing
  parenthesis.
- **Issue:** the brief said classify.rb knew none of the String rules;
  main had two of them since the run of 2026-11-13. The rest (getbyte,
  setbyte, "reads characters", "makes text", "is not text") were bugs or
  encodings.
  **Resolution:** two reasons, invalid-strings and binary-strings,
  checked before the encodings rule.
- **Issue:** status.tsv did not say whether a file was refused or
  crashed, which COMPATIBILITY.md needs.
  **Resolution:** a `stage` column.
- **Issue:** an example cameo passes and CRuby 3.3.6 does not run, one
  ruby/spec guards for Ruby 3.4, counted as pass.
  **Resolution:** it is harness, "not run under Ruby". 41 of the 990
  passes of 2026-11-13 are such examples.
- **Issue:** `make spec-status` pulled the latest ruby/spec, so its
  counts drifted from specs/, which was sorted at a fixed commit, and it
  needed the network.
  **Resolution:** run.rb asks checkout.rb for the commit COUNTS.md names;
  checkout.rb fetches only when the copy lacks it.
- **Issue:** a NUL in an example name made status.tsv a binary file for
  grep.
  **Resolution:** control characters are written as `\xNN`.
- **Issue:** `a.should output "x"` became an argument list Ruby does not
  parse.
  **Resolution:** a matcher call without parentheses is put in parentheses.

## Outcome
All of ruby/spec in 51 seconds on four cores. 3347 examples ran, 2897 of
them pass under CRuby in the harness, and cameo passes 1558 of those;
1198 are not expected to work, 141 are bugs, 450 are the harness's, and
2609 files did not run. COMPATIBILITY.md has the counts per category.

## Insights
- The CRuby the run compares with decides what counts: with 3.3.6 the
  examples ruby/spec guards for 3.4 do not count, though cameo follows
  3.4. A run with CRuby 3.4 would count them.
- Most files do not run because mspec's `before` blocks set instance
  variables (1465 files). A prelude that gives `before` somewhere to keep
  its state is the largest gain left in the harness.
