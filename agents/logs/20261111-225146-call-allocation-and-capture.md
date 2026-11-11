# Task: Cut the allocation of a call and of a block's capture

**Started:** 2026-11-11 22:51:46
**Ended:** in progress
**Strategy:** Performance
**Status:** In Progress
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** in progress

## Objective
Continue the performance work on the ir/ stack from the four areas its
CPU profiles named. Two were done and unmeasured (#367 mayMutate, #368
map literals); two were open: the allocation of every method and block
call, and the copy of every visible variable a block makes when it is
written. Then compare main's interpreter, the top of this stack and
CRuby on the benchmarks.

## Progress
- [x] Measured #367 and #368 on an Apple M4, put the tables in their
      Test sections and marked them ready
- [x] Call allocation: machines are reused, an operator on two integers
      or two floats makes no argument list, a lambda call's frame and
      scope are one allocation, a block and its body are one allocation
- [ ] Block capture
- [ ] The three-way benchmark

## Obstacles
- **Issue:** a CPU profile from runtime/pprof on macOS showed the
  interpreter's loop at 12% of app_fib and the Go scheduler and garbage
  collector at the rest, which GOGC=off disproved: the wall time did not
  change.
  **Resolution:** `sample <pid>` gives the functions on top of the main
  thread's stack; allocation was about half of them.
- **Issue:** the first run of a binary that was just built is slower by
  up to 0.3 s, which made the lambda case look 50% slower once.
  **Resolution:** every timing discards nothing but takes the fastest of
  five alternating runs.

## Outcome
in progress

## Insights
- With the garbage collector off the times stay the same on this
  machine: the collector runs on other cores. What a call costs is the
  allocation itself, and the clearing of what it allocates.
