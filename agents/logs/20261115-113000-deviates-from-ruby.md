# Task: state a difference from Ruby in one form in comments

**Started:** 2026-11-15 11:30:00
**Ended:** 2026-11-15 12:00:00
**Strategy:** Chore
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 40k input, 6k output

## Objective
A comment that says cameo differs from Ruby is to read "This deviates
from Ruby's behaviour: Ruby does foo, cameo does bar", not "…, where
Ruby does foo".

## Outcomes
- 21 comments in 17 files reworded, and the rule is in
  agents/WRITING.md under "Comments in code".
- Left as they were: comments that compare Go with Ruby to explain a
  correction that makes cameo do as Ruby does (uri's escaping, the zone
  names), since cameo does not deviate there.
