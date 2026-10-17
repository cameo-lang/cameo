# 35. `min` and `max` in one pass, integer sorts without dispatch

Step 36's comparison, run while this step was being written, showed one
case far out of line: `array_min--ary1000000.min` took 16 times MRI's time.
`min`, `max`, `minmax`, `min_by` and `max_by` were written in step 4 as
"sort the elements and take the first", which is correct and costs a sort
of a million elements for each call.

## One pass

Without a count, they now make one pass over the elements and keep the
first of equal ones, as Ruby does
([src/stdlib/enumerable.go:943](https://github.com/radiospiel/amber/blob/f719d1c06cc15998cf7dd9d36ad1efe3d2aadab0/src/stdlib/enumerable.go#L943-L958)). `max` and `minmax`
used to return the last of equal elements, so this is a fix as well as a
speed-up: `[[1, :a], [1, :b]].max_by(&:first)` is `[1, :a]` in Ruby, and
now in amber.

## Sorting integers

Sorting compared every pair through `<=>`'s dispatch. An array of Integers
in the `int64` form (step 25) now sorts by comparing the numbers directly,
since no class can redefine `Integer#<=>`, and `sort_by` with Integer keys
does the same ([src/stdlib/enumerable.go:745](https://github.com/radiospiel/amber/blob/f719d1c06cc15998cf7dd9d36ad1efe3d2aadab0/src/stdlib/enumerable.go#L745-L747)).

## Measured

Ten calls of each on a million integers ([35-minmax.arb](35-minmax.arb)),
on the binaries of steps 34 and 35:

| | `min` | `max` | `sort` |
|---|---|---|---|
| MRI | 0.010 s | 0.007 s | 1.09 s |
| step 34 | 27.2 s | 27.5 s | 27.2 s |
| step 35 | 0.072 s | 0.071 s | 20.5 s |

`min` and `max` are 380 times faster and within a factor of ten of MRI.
`sort` gains only a quarter and stays 19 times MRI's: the integer path
still sorts the Values with `sort.SliceStable`, whose merging moves each
element many times. A stable sort is not needed for integers, which are
equal only when they are the same number, so sorting their `int64`s with
an unstable sort is the next step for this case; the history ends before
it.
