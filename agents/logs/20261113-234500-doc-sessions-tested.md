# Task: run the shell sessions of the site and the README in a test

**Started:** 2026-11-13 23:45:00
**Ended:** 2026-11-13 00:15:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 90k input, 10k output

## Objective
The sessions with ^C, cameo resume and --io=file, and the README's irb session and Go excerpt, were run by hand. A test is to hold them to what cameo prints.

## Outcomes
- tests/sessions_test.go builds cameo and runs every one-line command of the site that has an output block, the README's irb and ^C sessions, and holds the README's Go to lines of examples/ex1/main.go.
- Each test was seen to fail on a document changed to say something false.
