# Task: one String constructor, which takes the encoding

**Started:** 2026-11-14 17:30:00
**Ended:** 2026-11-14 19:00:00
**Strategy:** Refactor
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 200k input, 20k output

## Objective
A String is to be made with its bytes and what they are, at every call:
no String of unknown kind, and text that knows whether it is ASCII.

## Outcomes
- `object.NewString(s, enc)` with ASCII, UTF8, USASCII and Binary takes
  the place of String, ASCIIString, Counted, Text and StringOf; the
  call sites were rewritten with `gofmt -r`. 88 are ASCII by
  construction (a literal, hex, base64, digits), 245 UTF8.
- UTF8 reads the text once: it counts the characters, and bytes that
  are not UTF-8 make a binary String. So `CGI.unescape` and
  `URI.decode_www_form_component` of such bytes return a binary String,
  where they returned a UTF-8 String that was not valid.
- `object.CheckASCII`, which the tests set, panics for a String given as
  ASCII that is not.
- The integration test of ^C failed once in three runs of `make check`
  and passed alone; it depends on when the interrupt lands.
