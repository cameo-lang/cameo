# Task: remove what the evaluator kept for compiled methods

**Started:** 2026-11-12 05:20:00
**Ended:** 2026-11-12 05:45:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 80k input, 6k output

## Objective
The compiler of methods to Go is gone. The evaluator still had what a
compiled method used, with nothing left to call it.

## Progress
- [x] `eval/aot.go` and its test: `ErrDeopt`, the registry of compiled
      programs, `StopFlag`, `InterruptFlag`, `Interrupted`, `ClassNamed`,
      `Construct`
- [x] A method no longer gets a Go implementation beside its body, when
      it is defined or restored from a dump
- [x] `dispatchTo`, `callSuperTo`, `runs` and `machine.call` no longer
      ask whether a method has both
- [x] The count of compiled methods running on the interpreter, and the
      flag a stop and a pause shared for compiled code
- [x] `make check`, the race tests, ruby/spec against the baseline

## Obstacles
- **Issue:** `dispatchTo` and `runs` tested for a method with a Go
  implementation and a body, and could not be simplified if anything
  else made one.
  **Resolution:** Nothing does: the two places that set both were the
  compiler's, and no code assigns `Native` or `Body` to a method after
  it is made.

## Outcome
A method is a Go function or the proc of a unit, and a call is a
builtin's or an activation. No behaviour of a script changes: ruby/spec's
3,939 status rows are the same.
