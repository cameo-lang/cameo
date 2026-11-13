# Task: measure ruby/spec again, and correct the dates of the benchmark run

**Started:** 2026-11-13 23:00:00
**Ended:** 2026-11-13 23:20:00
**Strategy:** Chore
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 60k input, 6k output

## Objective
The count of ruby/spec examples that pass was that of 2026-11-05. It is measured again, and every document that quotes it follows.

## Outcomes
- 990 of 2073 pass, where 1045 of 2134 did. All 57 examples lost stop at the rule that bytes which are not UTF-8 are a binary String; two examples pass that did not.
- classify.rb knows that rule and Integer#quo as decisions, so the bug rows are 61, not 133.
- The benchmark run was dated 2026-11-14 in three documents; it ran on 2026-11-13.
