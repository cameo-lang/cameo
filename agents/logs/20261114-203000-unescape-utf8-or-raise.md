# Task: what cgi and uri unescape is UTF-8 text, or an error

**Started:** 2026-11-14 20:30:00
**Ended:** 2026-11-14 20:50:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Low
**Used Models:** Fable 5.2
**Token usage (Estimated):** 40k input, 4k output

## Objective
`CGI.unescape` and its like are to return valid UTF-8 text always, and
to raise when the input gives bytes that are not UTF-8.

## Outcomes
- Every function of `CGI` that returns a String, and
  `URI.decode_www_form` and `decode_www_form_component`, raise
  Encoding::InvalidByteSequenceError for such bytes, through
  `stdlib.TextOf`. They returned a binary String since the constructor
  took an encoding, and an invalid UTF-8 String before that.
- `CGI.unescapeHTML("&#xD800;")`, a surrogate, raises too: Ruby writes
  its three bytes, which are no character.
