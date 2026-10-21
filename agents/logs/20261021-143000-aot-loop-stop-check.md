# Task: the stop check of a compiled loop

**Started:** 2026-10-21 14:30:00
**Ended:** 2026-10-21 15:15:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable
**Token usage (Estimated):** 40k input, 8k output

## Objective
Measure what the stop check costs a compiled loop, and make it cheaper if
it is more than a few percent.

## Progress
- [x] Baseline: tests/performance/03-blocks/times, thirty million passes, 0.062 s; 0.024 s with the check removed from the loop by hand
- [x] Two forms measured by hand: the loop's variable masked, 0.029 s; a local counter, 0.029 s
- [x] A loop on int64 checks on its own variable, one on *big.Int and a while on a local counter; the call counter in the state stays for calls
- [x] A counter per nested loop made triples 9% slower, register pressure in three loops deep; the loop variable costs nothing extra
- [x] Examples regenerated, tests, conformance 61 same, make check

## Obstacles
- **Issue:** `s.calls++` is a store through the state pointer on every pass; the compiler cannot keep it in a register across the loop.
  **Resolution:** Count on what the loop already increments.

## Outcome
times 0.062 s to 0.028 s; triples unchanged at 0.034 s.

## Insights
Measure the removed check first: it bounds what any cheaper form can gain,
here 0.024 s against 0.028 s reached.
