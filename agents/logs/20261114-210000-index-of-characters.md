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
- `src/str/index.go`, in a package of its own that knows no value, keeps, per String, the byte at which every
  64th character starts, as far as an index has asked. A lookup walks
  less than 64 characters from an entry.
- The index is made when a String is first indexed and kept for as long
  as its bytes are. A value has no room for one, so the indexes are in a
  table by the address and length of the bytes. An index holds a weak
  pointer, so it keeps no bytes alive, and the collector drops it after
  them. Strings under 256 bytes are walked.
- `weak.Make` is fatal for the program's own data, and
  `runtime.AddCleanup` panics for it, which is how a literal is told
  apart: its index stays.
- 20,000 characters with one `é`, read by index: 77 ms before, 2.5 ms
  now, CRuby 16.5 ms. One run on a laptop.
- Not measured: many workers indexing different long Strings at once
  share the table's lock.
