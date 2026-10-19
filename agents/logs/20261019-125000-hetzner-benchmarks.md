# Task: rerun the recorded benchmarks on 16 cores, and write down how

**Started:** 2026-10-19 12:50:00
**Ended:** 2026-10-19 15:20:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Medium
**Used Models:** Opus
**Token usage (Estimated):** 300k input, 25k output

## Objective
Measure the stack of async changes on a 16-core server: the full
benchmarks/compare sweep, with a baseline of main first, and fib at 1, 4
and 16 cores against CRuby 3.4 and 4.0 with and without their JITs.
Record the procedure as a skill.

## Progress
- [x] fib on main and each stack level at 1, 4 and 16 cores
- [x] provision.sh: Go, Rust, CRuby 3.4.11 and 4.0.7 from source
- [x] benchmarks/compare baseline on main, all four Rubies
- [x] benchmarks/compare on the stack, cameo alone
- [x] fib on the CRubies, server deleted
- [x] Skill, README, results, pull request

## Obstacles
- **Issue:** A clone of a git bundle has the branches only as
  origin/<branch>; `git checkout main` failed.
  **Resolution:** Check out origin/<branch> detached; the skill says so.
- **Issue:** Single cases of the compare suite moved by up to 70% between
  the two sweeps.
  **Resolution:** The three largest were timed again alone, main and branch
  alternately: equal. The suite's geometric mean moved by 0.7%.

## Outcome
benchmarks/compare: cameo 283% of MRI on main, 281% on the stack, 852
cases clean. Fine-grained async fib(28): 17.2 s to 1.48 s on one core,
24.4 s to 0.42 s on 16. The server ran about two hours.

## Insights
The recorded suite has no async case; the fib scripts are what show the
async changes.
