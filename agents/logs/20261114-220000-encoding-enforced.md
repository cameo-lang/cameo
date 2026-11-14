# Task: enforce a String's encoding where it is made

**Started:** 2026-11-14 22:00:00
**Ended:** 2026-11-15 01:00:00
**Strategy:** Feature
**Status:** Completed
**Complexity:** High
**Used Models:** Fable 5.2
**Token usage (Estimated):** 350k input, 40k output

## Objective
`NewString` is to read the bytes always and refuse those that are not
the encoding given, as an ArgumentError; a caller with bytes from
outside is to choose UTF8 or Binary; and the scan is to use vector
instructions.

## Outcomes
- `NewString` panics with `*object.WrongEncoding` for ASCII or US-ASCII
  with a byte above 127 and for UTF8 that is not UTF-8. `Interp.run`
  catches it outside the loop of instructions and raises ArgumentError
  in the script. A recover inside the loop made calls a third slower
  (`app_fib`, `vm_yield`); outside it the 781 benchmark cases are at
  0.99 of before, two runs each on a laptop.
- There is no constructor that takes either: `File.read` raises for a
  file that is not UTF-8 (`mode: "rb"` reads bytes), a host's `string`
  must be UTF-8, `JSON.parse` raises for a lone surrogate.
- The scan reads 16 bytes at a time: SSE2 on amd64, NEON on arm64 (the
  assembler has no name for UMAXV, which is given as a WORD), two words
  elsewhere. 55 GB/s for ASCII on an Apple M4.
- The count of characters is still recorded when a String is made: the
  scan that checks the encoding finds it, so recording it later would
  save nothing.
- `object.TrustedString(trusted.Key{}, s, enc, chars)` makes a String
  without reading it, for cameo's own code that made the bytes from
  checked Strings. `trusted` is an internal package, so a host cannot
  name the key; `examples/` cannot either. It takes the place of
  `StringOfChars`, which any importer could call.
- `str.UnknownLength`, -1, is the count of a String nobody counted:
  `Chars` returns it, where it returned a second value.
- Package `str` holds what is known of a string's bytes without a
  value: the encodings, `UnknownLength`, the scan with its assembly,
  the index of characters and which encodings go together. `object`
  imports it. Locals named `str` in the libraries are `strOf` now,
  since they hid the package.
