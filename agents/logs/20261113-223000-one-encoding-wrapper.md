# Task: one wrapper for encodings around a String method

**Started:** 2026-11-13 22:30:00
**Ended:** 2026-11-13 23:00:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 120k input, 8k output

## Objective
`InstallStringEncodings` wrapped each String method twice: once to run
it in the receiver's encoding, once to raise for a binary receiver
(issue #420). Each call went through both closures. Make it one.

## Progress
- [x] The first wrapper raises for a binary receiver itself
- [x] The methods defined again after it (the four case changes,
      `each_char`, `each_line`) raise themselves
- [x] The second loop wraps only the methods the first leaves alone
- [x] A test of 23 methods on a binary receiver, same output before and after
- [x] `make check`

## Outcomes
- 3,000,000 rounds of `upcase`, `strip`, `index` and `center` on a short
  string: 1.52 s before, 1.36 s after, best of three each.
- No behaviour changed.
