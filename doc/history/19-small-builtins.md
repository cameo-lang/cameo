# 19. `Math`, `Range#bsearch`, `Float#step` and other builtins

The last of the `stdlib/` decisions of step 13: a set of small methods the
benchmarks use, none of which needed a decision of its own.

- `String#codepoints`, `#byteindex`, `-@` and `+@`; `Enumerable#grep_v`;
  `Array#intersection` with several arrays; `yield_self`; `Hash#to_proc`,
  so `&hash` works; `Hash#flatten`.
- `Math` with its functions, `PI` and `E`, raising `Math::DomainError`
  outside a function's domain, as Ruby does: `Math.sqrt(-1)` raises rather
  than returning `NaN`.
- `Range#bsearch` follows CRuby's `range_bsearch`
  ([src/stdlib/range.go:386](https://github.com/radiospiel/amber/blob/@@COMMIT@@/src/stdlib/range.go#L386-L477)) over integer ranges,
  endless and beginless ones included, and over floats, in both of Ruby's
  modes: find-minimum, where the block returns true or false, and find-any,
  where it returns a number.
- `Numeric#step` computes each float from the start, `start + i * step`,
  as Ruby does, instead of adding the step to the previous value, so no
  rounding error accumulates over a long range: `1.0.step(2.0, 0.1)` ends
  at `2.0`, as in Ruby.

35 benchmark cases move to `works/`. The `bsearch` cases over integers
past 64 bits go to `unsupported/bignum`, and a `yield_self` without a
block, which returns an enumerator, to `unsupported/enumerators`.
