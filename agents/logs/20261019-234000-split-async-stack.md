# Task: rework #106 to #110 into pull requests of one concern each

**Started:** 2026-10-19 23:40:00
**Ended:** 2026-10-20 00:20:44
**Strategy:** Refactoring of the history; Bug Fix for the review findings
**Status:** Completed
**Complexity:** Medium
**Used Models:** Opus
**Token usage (Estimated):** 400k input, 60k output

## Objective
Rebuild the pull requests merged after 6ffe953 as a new stack on it, split
each into pull requests of one concern, run a code review on each, and fix
what the reviews confirm.

## Progress
- [x] #106 split in three: #111 (shared top level), #112 (values without
  the envelope), #113 (no run-wide locks)
- [x] #107 kept whole as #114, moved to the bottom of the stack
- [x] #108 kept whole as #115, #109 as #116
- [x] #110 split in two: #117 (procedure and scripts), #118 (results)
- [x] Code review of every pull request; fixes pushed as new commits
- [x] make check on every branch; go test -race on src/eval, src/cameo and
  tests on every branch that touches eval

## Obstacles
- **Issue:** #111 dropped the top-level snapshot before a class body ran
  and never for a top-level alias, so later workers saw a stale top level.
  **Resolution:** Drop it after each change; test fails on the original.
- **Issue:** #112 shared a field-less instance between goroutines, and
  ToMessage writes its cached message into the instance: a data race.
  Invalid UTF-8 strings skipped the envelope that rejects them.
  **Resolution:** The worker gets its own instance; such strings take the
  envelope. Both tests fail on the original.
- **Issue:** One go test -race run of src/cameo failed, with its output
  lost; 40 reruns passed.
  **Resolution:** TestRunContextStopsTheScript bounded a stop at 2 s and
  could hang; it now fails a case after 5 s instead.
- **Issue:** The skill of #110 lost a run in two places: a clone of its
  bundle has no origin/main, and its scp read fib.tsv from the laptop just
  before deleting the server. run.sh recorded failed runs as zero seconds.
  **Resolution:** Fixed in #117; run.sh tested on macOS with stand-ins for
  taskset and GNU time, not on a server.
- **Issue:** The fib README of #110 gave numbers its results.tsv contradicts
  (77-116 MB for 42-116 MB) and causes nothing measured.
  **Resolution:** Corrected in #118 against the committed data.
- **Issue:** gh pr edit fails for lack of the read:org scope.
  **Resolution:** Edited descriptions through gh api.
- **Issue:** The stack order put Benchmark between async changes it does
  not depend on.
  **Resolution:** Moved #114 to the bottom; the top tree did not change.

## Outcome
Eight pull requests, #114, #111, #112, #113, #115, #116, #117, #118, from
6ffe953 up. Open questions, listed in #114, #115 and #116: Benchmark as a
builtin constant without require, the lifetime of a promise whose id
travels, and how many runners the scheduler may start.

## Insights
Every split commit passed make check as it stood, so the original commits
were already one concern each; the reviews found the bugs, not the split.
