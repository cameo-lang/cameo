# Task: comments say a function returns, not reports

**Started:** 2026-11-13 22:00:00
**Ended:** 2026-11-13 22:30:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 150k input, 8k output

## Objective
agents/WRITING.md says a function returns a value and does not "report"
one. 94 comments under `src/` said "reports" (issue #419). Change them.

## Progress
- [x] "reports whether" is "returns whether", 73 comments
- [x] "X reports a Y" is "X returns true for a Y" or "returns whether",
      21 comments
- [x] "It reports false when" and the like, inside longer comments, 13
- [x] `make check`

## Outcomes
- Left as they are: comments where something is printed or shown to a
  person (an error the binary reports, minitest's report), and what Ruby
  reports. Those are not a function's return value.
- No code changed: every changed line is a comment.
