# Task: index a String without reading all of it

**Started:** 2026-11-14 14:00:00
**Ended:** 2026-11-14 16:30:00
**Strategy:** Bug
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 250k input, 25k output

## Objective
`s[i]` took time linear in the String (#435). It is to be fast, getbyte
is to read the byte where it is and to refuse a UTF-8 String that is not
ASCII, and a String is to know what it is without being read at run time.

## Progress
- [x] `s[i]` reads one character: from the bytes of an ASCII, US-ASCII or
      binary String, and by walking from the nearer end of any other
- [x] Every UTF-8 String counts its characters when it is made
- [x] `getbyte` raises for a UTF-8 String with characters beyond ASCII
- [x] `Array#join` gives the count it has, and reads no part twice

## Outcomes
- Options looked at: an ASCII path alone (18 of 56 makers of Strings gave
  a String that did not know it was ASCII, format and interpolation among
  them); counting in each maker (as many places to forget); counting in
  `object.String` (chosen: one place, and no case of the 781 benchmarks
  more than a quarter slower, once join stopped counting per part).
- Not done: an index into a String with characters beyond ASCII still
  walks to the character, 77 ms for 20,000 characters where CRuby takes
  16 ms. A table of offsets per String would make it constant.
- Measured on a laptop, two runs per case for the 781.
