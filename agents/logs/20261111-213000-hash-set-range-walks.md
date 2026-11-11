# Task: the walks of a Hash, a Set and a Range as steppers

**Started:** 2026-11-11 21:05:00
**Ended:** 2026-11-11 21:30:00
**Strategy:** Feature (TDD)
**Status:** Completed
**Complexity:** Simple
**Used Models:** Fable 5.2
**Token usage (Estimated):** 250k input, 15k output

## Objective
Step 23 of `doc/design/ir.md`, its second group: the walks a Hash, a Set
and a Range define themselves, which the count of the pull request
below showed still held a pause off.

## Progress
- [x] Hash: `each`, `each_pair`, `each_key`, `each_value`, `select`,
      `filter`, `reject`, `transform_values`, as walks over the keys
      that keep the values beside them
- [x] Set: `each`, `select`, `filter`, `reject`
- [x] Range: `each` over integers keeps the integer it yielded last, with
      or without an end
- [x] `integerCmp` and `integerAdd`, so a step of `upto` or of a range
      allocates nothing for an integer that fits 64 bits
- [x] Tests: every pause inside these blocks lands; paused and resumed
      output equals one run's

## Obstacles
- **Issue:** A walk calls its block with the element, and a Hash's with
  the key and the value.
  **Resolution:** A Hash's walk goes over the keys and keeps the values
  as a List in the first place of what it keeps, which its `yield`
  reads by the index.
- **Issue:** A range without an end cannot be walked as a list of
  elements.
  **Resolution:** `Range#each` is a stepper of its own, as `upto` is. A
  range of strings, or of a script's objects that have `succ`, is still
  walked to its end in one step.

## Outcome
A pause inside `hash.each { … }`, `set.each { … }` and `(1..n).each { … }`
lands at once. A step of a range of integers is cheaper than it was:
3,000,000 iterations of `each` and of `upto` and a `select` over 200,000
pairs run in 3.4 seconds here, 4.2 before.
