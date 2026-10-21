# Task: tests/performance, the compiled benchmarks against CRuby

**Started:** 2026-10-21 09:56:00
**Ended:** 2026-10-21 10:20:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable
**Token usage (Estimated):** 150k input, 15k output

## Objective
Give the compiled benchmarks a place of their own, one directory per step
of the compiler, and a runner that prints the comparison every step's pull
request shows: cameo's interpreter against CRuby, and compiled cameo
against YJIT, each as a difference from the Ruby it is compared with.

## Progress
- [x] benchmarks/aot moves to tests/performance/01-ints (fib, tak, tarai) and 02-bignums (fact)
- [x] run.rb in Ruby, in place of run.sh: one Markdown table, best of three, build output under bin/_performance
- [x] A Go test holds that every script compiles at least one method
- [x] Timed on the Mac mini M1; results in each directory's README
- [x] make check, pull request

## Obstacles
- **Issue:** run.sh printed every time as 0,000 on a machine with a German locale: bash's printf reads a decimal point only in the C locale.
  **Resolution:** The runner is Ruby, whose Float() and format do not read the locale.
- **Issue:** Build output inside the module is a package to `go vet ./...` and `go test ./...`.
  **Resolution:** It goes to bin/_performance; the Go tool ignores a directory whose name starts with an underscore, and builds it when named.

## Outcome
tests/performance with run.rb, two directories with their results on the M1, and the test that every script compiles.

## Insights
Timing needs an idle machine: a `go test` running beside the runner doubled the compiled fib's time.
