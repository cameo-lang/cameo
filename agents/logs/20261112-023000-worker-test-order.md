# Task: a worker test that depended on timing

**Started:** 2026-11-12 02:20:00
**Ended:** 2026-11-12 02:30:00
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 60k input, 4k output

## Objective
CI failed on the pull request of step 28 in
`TestAWorkerSomethingNeedsRunsToItsEnd`, a test of step 25: "the dump
holds 1 workers and 0 calls, want none".

## Progress
- [x] Reproduced: eight copies of the test binary at once, two CPUs each,
      400 runs each: 68 of 3,200 failed. Alone it passed 600 times.
- [x] Found the cause, in the test
- [x] The test orders the worker and the script with a promise
- [x] 3,200 runs under the same load pass; the whole package 200 times

## Obstacles
- **Issue:** The worker asked for the pause at its 500th turn, and the
  test assumed the script was inside the block of `gsub` by then,
  waiting for the worker. On a busy machine the worker got there first:
  the script paused before `gsub`, with the worker stopped, which is
  what a pause should do there.
  **Resolution:** The worker waits for a promise the script resolves
  inside the block, and asks for the pause after that.

## Outcome
The test holds the one order it is about. Both ways that order can go,
the worker stopped before the script waits for it or after, end with a
dump without workers.
