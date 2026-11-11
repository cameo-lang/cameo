# Task: Time.strptime reads %s and a zone name for %z

**Started:** 2026-11-11 14:00:00
**Ended:** 2026-11-11 14:40:00
**Strategy:** Bug Fix
**Status:** Completed
**Complexity:** Simple
**Used Models:** Claude
**Token usage (Estimated):** 1.5M input, 25k output

## Objective
Two CRuby benchmark cases stopped on cameo's `Time.strptime` and were filed
under `benchmarks/ruby/bugs/` (#320):

- `Time.strptime("1.5", "%s.%N")` raised `strptime: Go's time has no layout
  for %s`. CRuby returns `1970-01-01 00:00:01.5 +0000`.
- `Time.strptime("20010203 UTC", "%Y%m%d %z")` raised `invalid date or
  strptime format`. CRuby returns `2001-02-03 00:00:00 UTC`.

Fix both, move the two files back to `works/`, and keep `make check` and
`ruby run.rb works` green.

## Progress
- [x] Reproduced both with `bin/cameo` on the two benchmark files
- [x] Failing test first: `TestTimeStrptimeReadsEpochSecondsAndZoneNames`
      in `tests/time_test.go`, expectations taken from CRuby 3.3.6 under
      TZ=UTC; it failed on `%s` before the fix
- [x] Root cause, `%s`: `StrptimeLayouts` accepted `%s` only as the whole
      format or `%s %z` (a regular expression), and `ParseEpoch` split the
      string on whitespace; `%s.%N` matched neither
- [x] Root cause, `%z`: the directive translated to Go's `-0700` and
      `Z07:00` layouts only, and neither reads a name such as `UTC`
- [x] Fix in `src/stdlib/golayout.go`: a format starting with `%s` sets
      the Epoch field and its rest translates to layouts as any format
      does, limited to a fraction and a zone; `ParseEpoch` reads the
      seconds, then the rest by those layouts, and adds the fraction away
      from zero as CRuby does. `%z` gets a third layout, `MST`, for a zone
      name; `fixZone` already maps the names Ruby knows to their offsets
- [x] Callers: `src/lib/time/time.go` keeps the nanoseconds;
      `src/lib/date/goparse.go` sets `seconds` and `sec_fraction` as Ruby's
      `Date._strptime` does (-1 and 1/2 for "-1.5"), and `rewriteFrags`
      drops a text's `sec_fraction`, as `rt_rewrite_frags` does, so
      `DateTime.strptime` matches CRuby
- [x] `TestDateStrptimeReadsEpochSecondsAndZoneNames` in
      `tests/date_test.go`
- [x] Moved both files to `works/`, MANIFEST rows back to `works` with an
      empty reason, README sentence about `bugs/` updated
- [x] `ruby run.rb works`: exit 0, 798 files clean; `make check`: exit 0;
      `make conformance`: exit 0, 897 scripts, none differ

## Obstacles
- **Issue:** On main the two files were still under `works/`; the move to
  `bugs/` is in the open pull request #320.
  **Resolution:** Branched from #320's head, as the one stack in flight.
- **Issue:** The test process sets `time.Local = time.UTC`, so a local
  time inspects as `UTC` there where CRuby under TZ=UTC prints `+0000`.
  **Resolution:** The test prints `strftime("... %z")` and checks `utc?`
  only for the zone-name cases.

## Outcome
Both formats read as CRuby reads them, for Time and for Date/DateTime;
both benchmark cases are back in `works/`. `Date._strptime` prints the
fraction as a Float where Ruby prints a Rational, which cameo leaves out.
A `%s` after another directive (such as `%Y %s`) is still an error.

## Insights
Go's layout parser treats a run of spaces in the layout as one or more
spaces in the input, so a layout read after the epoch seconds needs no
trimming.
