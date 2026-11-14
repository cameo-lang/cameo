# Task: an index of characters, so that a loop by index is linear

**Started:** 2026-11-14 21:00:00
**Ended:** 2026-11-14 21:40:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 60k input, 8k output

## Objective
`s[i]` on a String with characters beyond ASCII walked to the character,
so a loop over the String by index was quadratic. An index built as far
as i is to make the loop linear.

## Outcomes
- `src/stdlib/charindex.go` keeps, per String, the byte at which every
  64th character starts, as far as an index has asked. A lookup walks
  less than 64 characters from an entry.
- A value has no room for an index and is immutable, so the indexes of
  the eight Strings indexed last are kept beside them, under one lock,
  by the address and length of their bytes. Strings under 256 bytes are
  walked.
- 20,000 characters with one `é`, read by index: 77 ms before, 2.5 ms
  now, CRuby 16.5 ms. One run on a laptop.
- Not measured: many workers indexing different long Strings at once
  share the lock and the eight places.
