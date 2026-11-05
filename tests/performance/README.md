# Performance of compiled cameo

Scripts that time one thing cameo's compiler, `cameo build`, takes on, one
directory per step of [doc/design/aot.md](../../doc/design/aot.md), in the
order the steps landed. Each directory's README says what its scripts
exercise and holds the table of the run that landed the step.

| Directory | What it times |
|---|---|
| [01-ints](01-ints) | recursion over Integers that fit 64 bits: fib, tak, tarai |
| [02-bignums](02-bignums) | recursion whose product outgrows 64 bits: factorial |
| [03-blocks](03-blocks) | blocks of `times` and `each` over a Range as loops: a sum, Pythagorean triples |
| [04-floats](04-floats) | Float arithmetic in Integer loops: a Mandelbrot grid, the Leibniz series |
| [05-classes](05-classes) | methods of classes with typed fields: the Mandelbrot grid held in fields, vectors read through `ref` fields |
| [06-construction](06-construction) | `Class.new` in compiled code: a walk building vectors, two bodies rebuilt every step |

[run.rb](run.rb) builds cameo and each script, runs every case three times
on four Rubies and prints the best in-script seconds as one Markdown table:

```sh
ruby tests/performance/run.rb                      # every directory
ruby tests/performance/run.rb tests/performance/01-ints
ruby tests/performance/run.rb --ruby /opt/homebrew/opt/ruby/bin/ruby --runs 5
ruby tests/performance/run.rb --baseline origin/main   # also before and after a change
```

`--baseline REV` adds the time of each script compiled by cameo at REV,
the base of a pull request, and the compiled time's difference from it, and
prints each directory's speed-up over REV, the geometric mean of the
scripts' before / after, to stderr. A pull request that makes compiled code
faster shows its gain this way; at a REV without `cameo build`, the before
is REV's interpreter.

| Column | What ran |
|---|---|
| cameo interp | cameo's interpreter, `bin/cameo script.rb` |
| ruby (100%) | CRuby without YJIT, `ruby --disable-yjit`, which the interpreter's Δ compares with |
| cameo compiled | the program `cameo build` wrote, built with `go build` |
| ruby yjit (100%) | CRuby with YJIT, `ruby --yjit`, which the compiled Δ compares with; "-" when the Ruby has no YJIT |

A script Ruby cannot run, one with cameo's typed fields, gets "-" in the
Ruby columns and the error on stderr.

Δ is cameo's time as a difference from the Ruby to its left: 🟢 under -10%,
🟡 within ±10%, 🔴 over +10%. The scripts read the clock themselves and
print `result seconds` last, so no process start is in a time, and YJIT's
times include its warm-up. The runner fails when the four runs print
different results.

Every script compiles at least one method; `tests/performance_test.go`
holds that, so that a change to the compiler cannot turn a table into the
interpreter timed twice.

## Results

Mac mini (Apple M4, 10 cores), Go 1.26.4, CRuby 3.4.1, on 2026-11-05, at the
head of the stack that replayed the work since v0.4.2 (#293). Seconds,
best of three:

| script | cameo interp | ruby 3.4.1 (100%) | Δ | cameo compiled | ruby 3.4.1 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 01-ints/fib | 1.125 | 0.188 | 🔴 +500% | 0.011 | 0.029 | 🟢 -62% |
| 01-ints/tak | 1.996 | 0.282 | 🔴 +607% | 0.018 | 0.046 | 🟢 -61% |
| 01-ints/tarai | 1.569 | 0.226 | 🔴 +596% | 0.013 | 0.034 | 🟢 -62% |
| 02-bignums/fact | 0.355 | 0.379 | 🟡 -6% | 0.151 | 0.371 | 🟢 -59% |
| 03-blocks/times | 3.401 | 1.103 | 🔴 +208% | 0.016 | 0.939 | 🟢 -98% |
| 03-blocks/triples | 1.217 | 0.351 | 🔴 +247% | 0.019 | 0.241 | 🟢 -92% |
| 04-floats/mandelbrot | 1.740 | 0.544 | 🔴 +220% | 0.013 | 0.192 | 🟢 -93% |
| 04-floats/pi | 1.603 | 0.788 | 🔴 +103% | 0.010 | 0.379 | 🟢 -97% |
| 05-classes/mandelbrot | 1.925 | - | - | 0.013 | - | - |
| 05-classes/pairs | 7.838 | - | - | 0.074 | - | - |
| 06-construction/bodies | 2.760 | - | - | 0.527 | - | - |
| 06-construction/vectors | 1.772 | - | - | 0.173 | - | - |

Compiled, every script is faster than YJIT: the Integer recursions and
the factorial, which is bignum multiplication in Go's `math/big` either
way, take two fifths of its time, and the loops over blocks and Floats a
twelfth to a sixtieth. The interpreter takes from CRuby's own time without
YJIT, on the factorial, to seven times it, on the Integer recursions.
Each directory's README holds the table of the run that landed its step,
on the Mac of that day; this one is the whole suite on one day.
