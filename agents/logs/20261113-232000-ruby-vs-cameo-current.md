# Task: bring the site's Ruby vs cameo page up to what cameo does today

**Started:** 2026-11-13 23:20:00
**Ended:** 2026-11-13 23:45:00
**Strategy:** Chore
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 80k input, 8k output

## Objective
The page was renamed and not reread. Each claim of its tables is run against the interpreter, and what the reference lists and the page lacked is added.

## Outcomes
- New: a block captures values, Strings are UTF-8 or binary, smaller differences; each with a sample the site test runs.
- Corrected: 781 benchmark cases, not 852; Ruby 3.4, not 3.3; exit! is refused by the checker; File reads and writes once the host allows files.
