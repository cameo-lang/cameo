# 25. Integers of arbitrary precision

Until this step an amber Integer was a Go `int64`, and arithmetic wrapped
when it left 64 bits. Ruby's integers grow without limit, and the
benchmark suite noticed: `app_factorial` computes `5000!`, and amber
printed 0. Worse than a crash, those cases exited cleanly with a wrong
result, which is why step 13 filed them under `unsupported/bignum`. On the
binary of step 24:

```
p (1..30).reduce(:*), 2**100        # -8764578968847253504, 0
```

and on this step's, as in Ruby:

```
p (1..30).reduce(:*), 2**100        # 265252859812191058636308480000000, 1267650600228229401496703205376
```

## Two forms, one number

An Integer has two forms: a `KindInt` holding an `int64` in the Value's
payload (step 12), and a `KindBig` pointing at a Go
[`big.Int`](https://pkg.go.dev/math/big#Int) once arithmetic leaves 64 bits
([src/object/bigint.go:9](https://github.com/radiospiel/amber/blob/4a2d224dc172b77bd41399f81d6dabd82c7246ee/src/object/bigint.go#L9-L21)). A result that
fits moves back to the small form, so a number has exactly one form:
equality and hash keys never compare a small 5 with a big 5, and the fast
paths of steps 9 to 11 keep seeing the integers they were written for.

The fast paths check for overflow and hand that one case to the ordinary
call, which computes with `big.Int`. The checks are a few instructions
each ([src/object/bigint.go:108](https://github.com/radiospiel/amber/blob/4a2d224dc172b77bd41399f81d6dabd82c7246ee/src/object/bigint.go#L108-L113)): an addition
overflowed when the sign of the result differs from the signs of both
operands.

## What it cost

The overflow checks are on the hottest path of the evaluator, so the
history benchmarks show them:

| | loop | fib |
|---|---|---|
| step 24 | 0.49 s | 0.48 s |
| step 25 | 0.59 s | 0.43 s |

The arithmetic loop, nothing but checked operators, pays about a fifth;
`fib`, dominated by calls, does not notice.

## Everywhere an integer goes

Integer's methods take either form, and comparison with a float is exact,
not through a float conversion that would lose digits. Literals,
`Integer()`, `to_i`, `hex`, `oct` and `format` read and print big numbers;
ranges iterate, count, sum and `bsearch` over big bounds. A big integer
travels in the envelope as `big_value`, its decimal digits, and the Go host
API gives and takes `*big.Int`; an `int` field is a protobuf `int64`, so it
refuses one with a `RangeError` instead of truncating.

`unsupported/bignum` is gone: 12 cases move to `works/`, the literal cases
to `unsupported/complex`, and `vm_case_lit` to
`unsupported/instance-variables`.
