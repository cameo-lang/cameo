# Task: examples for pausing a run and for file I/O

**Started:** 2026-11-14 10:20:00
**Ended:** 2026-11-14 11:00:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 200k input, 10k output

## Objective
The site is to show pause and resume from Go, and how a Go program
allows I/O, with programs a reader can run. Add them as examples with
tests, so the pages show files that are tested.

## Progress
- [x] `examples/pause`: pauses a run on its third step, dumps it,
      resumes it in a second program; the test resumes the dump twice
- [x] `examples/io`: a file system in memory, and a read-only one that
      refuses the script's write

## Obstacles
- **Issue:** The resumed program raised "uninitialized constant Host".
  **Resolution:** A dump holds nothing of the host: the module is
  defined again on the program `Resume` returns, and the example says so.
