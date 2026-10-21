# Task: compiled code checks the run's stop at every call and loop pass

**Started:** 2026-10-21 21:55:00
**Ended:** 2026-10-21 22:45:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 120k input, 15k output

## Objective
A stop reaches compiled code at its next call or loop pass, not after up
to 4096 of them, at no cost to the compiled benchmarks.

## Progress
- [x] Measured on an Apple M4: atomic.Bool.Load 0.25 ns, time.Since 12 ns,
  time.Now 28 ns; a plain and an atomic load of an array cost the same
- [x] Baseline: tests/performance scripts compiled, best of 10
- [x] state holds the stop flag; calls and loop passes read it; the call
  and pass counters are gone; eval.Stopped becomes eval.StopFlag
- [x] Examples regenerated, generator test updated, design note
- [x] make check, pull request

## Obstacles
- **Issue:** A 16 ms time slice needs a clock read, and a clock read every
  4096 events does not bound the time between them.
  **Resolution:** Read the flag on every event; no clock.
- **Issue:** The timing script rebuilt bin/cameo from the stashed baseline,
  and a later cameo build used it.
  **Resolution:** Rebuilt bin/cameo before generating code.

## Outcome
Compiled, best of 10: fib -48%, tak -38%, tarai -46%; loops and the rest
within ±5%. Code generated before this change calls eval.Stopped and has
to be generated again.

## Insights
The per-call counter was a store through the state pointer on every call;
removing it, not the flag read, is what made recursion faster.
