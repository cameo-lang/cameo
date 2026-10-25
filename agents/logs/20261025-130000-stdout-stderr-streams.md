# Task: $stdout and $stderr as the script's streams

**Started:** 2026-10-25 13:00:00
**Ended:** 2026-10-25 13:40:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 30k input, 8k output

## Objective
A script's progress messages go to its standard error, which Ruby
scripts write as `$stderr.puts`; cameo rejected `$stderr` as a global
variable, so such a script could not run on cameo at all.

## Progress
- [x] syntax: `$stdout` and `$stderr` read as the constants STDOUT and
      STDERR; assigning them is refused with a message of its own
- [x] stdlib: IO with puts, print, write, flush, sync, fileno, tty?
- [x] test, docs (cameo-vs-ruby, the site, the design)

## Obstacles
- **Issue:** `$stdout << "x"` is refused by the checker, which takes `<<`
  for a mutator of a receiver it cannot rebind.
  **Resolution:** IO has no `<<`; print does the same. Documented.

## Outcome
`make check` green: Go tests and 25 integration tests.
