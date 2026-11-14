# Task: setbyte for binary Strings alone

**Started:** 2026-11-14 17:00:00
**Ended:** 2026-11-14 17:20:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 30k input, 3k output

## Objective
setbyte is to set a byte of a binary String and to raise for any other.

## Outcomes
- `setbyte` raises Encoding::CompatibilityError for a UTF-8 and a
  US-ASCII String. This is unlike Ruby; doc/cameo-vs-ruby.md says so.
- A script reaches the method only with mutation errors ignored, or
  where the call rebinds a local; the test runs with them ignored.
