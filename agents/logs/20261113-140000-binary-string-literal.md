# Task: a literal for binary strings, b"..."

**Started:** 2026-11-13 12:10:00
**Ended:** 2026-11-13 14:30:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 1.2M input, 60k output

## Objective
A string literal with bytes that are not UTF-8 needs an explicit syntax,
`b"..."`. The change belongs with the pull requests of the stack that
decide what a string's bytes may be, not on top of it.

## Progress
- [x] `b"..."` and `b'...'` make a binary string; a plain literal with
      bytes that are not UTF-8 is a syntax error that names the literal
- [x] The examples and tests that wrote such bytes write `b"..."`
- [x] The two benchmark cases that write them moved to
      `unsupported/invalid-strings`
- [x] The pull request that makes a UTF-8 string hold whole characters
      alone, the next one up, no longer makes a literal binary by itself

## Obstacles
- **Issue:** The parser is a dependency and reads `b"x"` as Ruby does,
  as a call of `b`. Its tree does not tell `b"x"` from `b "x"`.
  **Resolution:** Its lexer gives each token a line, a column and
  whether a space came before it. The literals are found in the tokens
  before the parser runs and rewritten to a call of a reserved name;
  lines stay the lines they were. A parse error that names the reserved
  name names `b` instead.
- **Issue:** Below the pull request that forbids them, a UTF-8 string
  with invalid bytes still exists, and examples show it. A literal made
  it there.
  **Resolution:** In this pull request those examples write
  `b"...".force_encoding("UTF-8")`; the next one, which forbids such a
  string, changes them as it did before.
- **Issue:** `string_valid_encoding_p--valid_enc_false` and `_true` of
  CRuby's benchmarks failed on the top of the stack already: their
  prelude calls bytes UTF-8 that are not.
  **Resolution:** They are under `unsupported/invalid-strings` from this
  pull request on, where their literal stops them first.

## Outcome
One pull request below the one on valid strings, and that one amended.
`make check` passes on both.

## Left open
- `specs/supported` holds 66 files of ruby/spec that write such bytes in
  a plain literal, so `cameo check` refuses them now. The sorting in
  `specs/` was last made on 2026-10-29 against another commit of
  ruby/spec, and a new sort moves about 320 files for reasons older than
  this change; it is a task of its own.
- A `b"..."` inside the interpolation of a regexp literal is a call of
  `b`: the lexer gives a regexp as one token.
