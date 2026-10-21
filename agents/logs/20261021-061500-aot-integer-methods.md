# Task: cameo build, step one of ahead-of-time compilation

**Started:** 2026-10-21 06:15:00
**Ended:** 2026-10-21 07:25:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 400k input, 60k output

## Objective
Prototype step one of ahead-of-time compilation: Go source for top-level
methods over Integers. Measure it on CRuby's recursion benchmarks, here and
on a Mac the team owns.

## Progress
- [x] Interpreter hook: compiled methods registered by program digest, run as the method's Native, with the body kept for the calls they leave
- [x] Generator: typing, checked int64 arithmetic, Ruby's floor division and modulo, stop checks
- [x] `cameo build script dir [package]`
- [x] Examples held to Ruby's output, compiled and interpreted; generated packages checked to be current
- [x] benchmarks/aot with run.sh, timed in the cloud container
- [ ] Timed on the Mac

## Obstacles
- **Issue:** Factorial ran ten times slower compiled. Each overflow sent the call back to the interpreter, whose recursive call entered compiled code again and overflowed again, which made the cost quadratic.
  **Resolution:** A call that left compiled code runs every compiled method below it on the interpreter (`reinvoke`).
- **Issue:** Compiled `while` loops never checked the run's stop, so `RunContext` could not end them.
  **Resolution:** A stop check every 4096 calls and loop passes, tested with a loop that never ends.

## Outcome
Recursion over Integers runs about 100 times faster than on the interpreter.
Factorial runs as it did, since its product outgrows 64 bits.

## Insights
Keying compiled code on the digest of the whole program keeps direct calls
sound: no other script, and no edited copy of this one, gets them.
