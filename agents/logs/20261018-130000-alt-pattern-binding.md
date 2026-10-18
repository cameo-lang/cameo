# Task: `in A | B => name` binds the whole alternation (#96)

**Started:** 2026-10-18 13:00:00
**Ended:** 2026-10-18 13:15:00
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 20k input, 4k output

## Objective
`case 1.5; in Float | Integer => r` left r unset; Ruby binds 1.5.

## Progress
- [x] Reproduced; examples/patterns.cameo extended, expected output from ruby 3.3
- [x] Cause: go-ruby-parser v0.11.1 parses `A | B => r` as `A | (B => r)`
- [x] The converter rebuilds it as `(A | B) => r`

## Obstacles
- **Issue:** the cause is in the parser, a dependency.
  **Resolution:** the converter corrects the tree; the parenthesised form
  `(A | B) => r`, which the parser rejects, stays unsupported.

## Outcome
One pull request, fixing #96.
