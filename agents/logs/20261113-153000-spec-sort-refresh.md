# Task: sort ruby/spec again

**Started:** 2026-11-13 15:00:00
**Ended:** 2026-11-13 15:45:00
**Strategy:** Refactoring
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 400k input, 20k output

## Objective
`specs/` held the sorting of 2026-10-29. Since then the checker refuses a
string literal with bytes that are not UTF-8 and a block that rebinds a
variable around it, and cameo builds in many of the libraries the specs
require. Sort again, so that `supported/` is what `cameo check` accepts.

## Progress
- [x] sort.rb asks cameo whether it has a library, where it kept a list
- [x] Three messages of the checker have a directory
- [x] Sorted the copy of ruby/spec at `47b10f1d`, the commit
      `specs/checkout.rb` pulled
- [x] The counts in doc/cameo-vs-ruby.md, doc/stdlib.md and the README

## Obstacles
- **Issue:** The list of libraries in sort.rb named `set`, `time`, `pp`,
  `pathname` and `cgi`. cameo has had `date`, `uri`, `stringio`, `csv`,
  `zlib` and some twenty more since, and 641 specs were filed under
  `unsupported/require` for a library that loads.
  **Resolution:** sort.rb runs a script that requires the library, once
  per name. The list cannot go stale again.
- **Issue:** `cameo check` accepts a `require` of any name, so the
  checker cannot answer the question; the script has to run.
  **Resolution:** As above.
- **Issue:** `make spec-status` stops in this container: run.rb reads the
  checker's output in the locale's encoding and calls `strip` on it.
  **Resolution:** Not part of this change. Run under `LC_ALL=C.UTF-8` it
  finishes; status.tsv is left as it was, see below.

## Outcome
3858 spec files, 1855 in `supported/` (1403 of 3841 before). 1021 use an
instance variable, 375 require a library cameo lacks (1006 before), 71
write bytes that are not UTF-8 in a plain string literal.

## Left open
- `status.tsv` and doc/coverage.md are of the old sorting. A run under
  `LC_ALL=C.UTF-8` with CRuby 3.3.6 gives 990 passing examples where
  status.tsv has 1045. The 27 that passed before and do not now stop on
  the rules for strings (bytes that are not UTF-8 raise, a binary string
  has no characters); `runner/classify.rb` does not know those messages
  and calls them bugs. Four files stop on a parse error in what run.rb's
  rewrite makes of `instance_exec(3, &curried).should == 6`.
