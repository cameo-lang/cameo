# Task: faster step functions, sort_by, interpolation, literals, string length

**Started:** 2026-11-13 09:00:00
**Ended:** 2026-11-13 19:00:00
**Strategy:** Performance
**Status:** Completed
**Complexity:** Medium
**Used Models:** Fable 5.2
**Token usage (Estimated):** 3M input, 90k output

## Objective
Close the largest gaps to CRuby that the benchmark suite shows, one
pull request per cause, on top of the stack that ends in #388.

## Progress
- [x] #389 step functions: walks and counts the evaluator runs itself,
  the block's activation reused, the block run inline
- [x] #390 `Hash#each` gives a one-parameter block the pair
- [x] #392 `sort_by` sorts keys and indexes, as a step function
- [x] #393 interpolation adds each part to the string in place
- [x] #394 a call that is a statement runs as any call
- [x] #395 a string known to be ASCII reads its length off its bytes
- [x] #396 `values_at` takes a range's elements as one piece
- [x] #397 a list or a map of literals is made once
- [x] #398 a string keeps the number of its characters
- [x] #399 more makers of strings give the number
- [x] #400 a string is at most 4 GiB
- [x] #401 a substitution has its number from its parts
- [x] #402 tr, squeeze, padding, prefixes and suffixes, parts of ASCII
- [x] #403 a binary string answers its byte methods alone
- [x] #404 a UTF-8 string holds whole characters alone
- [x] #405 a US-ASCII string holds 7-bit characters alone

## Measurements
Suite geomean against CRuby: 172% at #388, 164% at #389, 153% at #394;
not run for #395 to #398. Single cases, seconds, before, after, CRuby:

| case | before | after | CRuby |
|---|---|---|---|
| `each`, 20M block calls | 1.209 | 0.271 | 0.287 |
| `sort_by` on integers | 1.582 | 0.436 | |
| `sort_by { length }`, long ASCII strings | 2.792 | 0.318 | 0.237 |
| `length` of long non-ASCII strings | 1.692 | 0.014 | 0.113 |
| `length` of sums and parts of non-ASCII strings | 7.358 | 0.430 | 0.610 |
| `values_at(1..2000)` | 0.573 | 0.154 | 0.078 |
| hash literal of 8 entries | 0.756 | 0.064 | 0.196 |
| interpolation of three integers, 30M times | 4.21 | 4.57 | 1.26 |

## Findings
- A `for … range` over an iterator with a `return` inside moves the
  enclosing function's results to the heap on every call; a helper that
  is not inlined avoids it.
- A string value has no object of its own, so a count made on first use
  has nowhere to stay: the value that was asked is a copy. The count is
  kept only when the string is made with it.
- Keeping the count through an interpolation costs 8% on a loop that
  does nothing else.

## Left open
- A string whose maker gives no count (`format`, `succ`, a part
  `split` or `scan` cut out of a string that is not ASCII, a file that
  was read, what a StringIO reads) is counted at every `length`.
- `String#[]` still decodes the whole string, known ASCII or not.
- The call path, `sort` with a block, `max` and `min` on integers.
