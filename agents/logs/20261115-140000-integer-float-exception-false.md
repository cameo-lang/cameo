# Task: Integer() and Float() honour exception: false

**Started:** 2026-11-15 14:00:00
**Ended:** 2026-11-15 14:30:00
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Opus
**Token usage (Estimated):** 60k input, 8k output

## Objective
`Integer("x", exception: false)` and `Float("x", exception: false)` raised
ArgumentError where Ruby returns nil. The keyword was ignored: any keyword
passed to either function was dropped without a check.

## Progress
- [x] Reproduced with `p Integer("x", exception: false)`: ArgumentError.
- [x] Checked the expected results against Ruby 3.3.6, edge cases included:
  nil, an Array, NaN, a `to_int` that raises, `exception: nil`, an unknown
  keyword.
- [x] Wrote tests/kernel_conversion_test.go; both tests failed.
- [x] Moved Integer()'s body into `toInteger` beside `toFloat`, and put both
  behind `convert`, which reads the keyword.
- [x] `make check` passed.

## Obstacles
- **Issue:** Ruby returns nil also when the value's own `to_int` raises.
  **Resolution:** `convert` returns nil for any StandardError the conversion
  raises when exception: is false.

## Outcome
Both functions take `exception:`; false returns nil for a value that does not
convert, true or no keyword raises as before, any other value or keyword
raises ArgumentError with Ruby's message. agents.md and doc/cameo-vs-ruby.md
did not mention the gap, so they are unchanged.
