# Recursion past 64 bits

Factorial of 5000, a hundred times, as CRuby's app_factorial. The product
outgrows 64 bits after 20 steps, and compiled code runs on int64: every
call of `fact` leaves the computation to the interpreter, so the compiled
build runs as the interpreter does.

| Script | What it runs |
|---|---|
| [fact.rb](fact.rb) | `fact(5000)` a hundred times, printing the digits of the result |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, cameo c3e80a7, CRuby 4.0.7, on
2026-10-21. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 02-bignums/fact | 0.421 | 0.770 | 🟢 -45% | 0.437 | 0.759 | 🟢 -42% |

The work is bignum multiplication, which cameo's interpreter does in Go's
`math/big` and CRuby in its own code; the compiled build adds the cost of
one call into Go per `fact` before the interpreter takes over.
