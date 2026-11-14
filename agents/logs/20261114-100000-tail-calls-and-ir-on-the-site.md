# Task: show tail calls and the IR on the site

**Started:** 2026-11-14 10:00:00
**Ended:** 2026-11-14 11:00:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 120k input, 12k output

## Objective
The site is to show a recursive length of a String with and without a
tail call, with what was measured, and to explain the IR and `cameo ir`
on a script that defines a class. The pages name v0.5.0 as the release.

## Outcomes
- Ruby vs cameo has two new sections under "Only in cameo": tail calls,
  whose sample the site test runs, and the IR, whose listing the session
  test holds to what `cameo ir` prints.
- Measured on a laptop, one run each: the tail call is a third faster
  and has no depth limit; CRuby is 8 to 10 times faster where it runs.
- Found on the way and not looked into: `s[i]` on a String of 5000
  characters makes the same recursion take 42 s, where CRuby takes
  0.55 s.
- agents.md says to end a recursion in the call, and names `cameo ir`.
