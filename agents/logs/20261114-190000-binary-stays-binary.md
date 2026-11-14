# Task: binary joined with anything is binary, and text refuses it

**Started:** 2026-11-14 19:00:00
**Ended:** 2026-11-14 20:00:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 120k input, 12k output

## Objective
Mixing a binary String into another is always to give a binary String.
What makes text for a reader is to refuse one: the decision named
interpolation and format, sprintf and %.

## Outcomes
- `+`, `<<`, `concat`, `prepend`, `insert` and `Array#join` return a
  binary String as soon as one part is binary. Unlike Ruby in two cases:
  an ASCII binary part gave UTF-8, and two parts with other bytes raised.
- An interpolation and format raise Encoding::CompatibilityError for a
  binary String. `puts` writes the bytes as before, and a method of text
  given a binary argument raises as before, for a byte above 127 alone:
  both were left out of the decision.
- No case of benchmarks/ruby/works fails under the rules.
