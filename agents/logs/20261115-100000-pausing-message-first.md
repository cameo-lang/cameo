# Task: find why the test of ^C fails now and then

**Started:** 2026-11-15 10:00:00
**Ended:** 2026-11-15 11:30:00
**Strategy:** Bug
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 90k input, 8k output

## Objective
An integration test of ^C failed in about one `make check` in four after
a change of code. The guess was that a new binary starts slowly.

## Outcomes
- The guess was wrong. A new binary's first run takes 300 ms where a
  later one takes 7 ms, but the test passes as its first run; cameo
  starts in at most 40 ms while everything compiles; the test passed 60
  times alone and 25 under load. The tests run a built binary, not
  `go run`.
- The test that failed was not the one assumed, but the one of a script
  that cannot be dumped. Its output had "cameo: pausing" after
  "cameo: the script goes on": the command asked for the pause and then
  said so, and the script could stop and answer in between.
- The command says "pausing" first now. Under load the old order gave
  the messages out of order in 1 of 300 interrupts, the new in 0 of 300.
- Not explained: why it showed in one check in four after a change and
  in none of 34 without one.
