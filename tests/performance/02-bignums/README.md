# Recursion past 64 bits

Factorial of 5000, a hundred times, as CRuby's app_factorial. The product
outgrows 64 bits after 20 steps. Compiled code runs on int64 and every
method has a twin on `*big.Int`: the overflow ends the int64 run of the
call, and the twin runs it from the start, as Go.

| Script | What it runs |
|---|---|
| [fact.rb](fact.rb) | `fact(5000)` a hundred times, printing the digits of the result |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, CRuby 4.0.7, on 2026-10-21.
Seconds, best of three.

With the twin on `*big.Int`:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 02-bignums/fact | 0.426 | 0.768 | 🟢 -45% | 0.208 | 0.759 | 🟢 -73% |

Before it, at c3e80a7, when every call of `fact` went back to the
interpreter:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 02-bignums/fact | 0.421 | 0.770 | 🟢 -45% | 0.437 | 0.759 | 🟢 -42% |

The work is bignum multiplication, in Go's `math/big` either way; the twin
saves the interpreter's dispatch of 5000 calls and 15000 operators per
factorial.
