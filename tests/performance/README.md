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

[run.rb](run.rb) builds cameo and each script, runs every case three times
on four Rubies and prints the best in-script seconds as one Markdown table:

```sh
ruby tests/performance/run.rb                      # every directory
ruby tests/performance/run.rb tests/performance/01-ints
ruby tests/performance/run.rb --ruby /opt/homebrew/opt/ruby/bin/ruby --runs 5
```

| Column | What ran |
|---|---|
| cameo interp | cameo's interpreter, `bin/cameo script.rb` |
| ruby (100%) | CRuby without YJIT, `ruby --disable-yjit`, which the interpreter's Δ compares with |
| cameo compiled | the program `cameo build` wrote, built with `go build` |
| ruby yjit (100%) | CRuby with YJIT, `ruby --yjit`, which the compiled Δ compares with; "-" when the Ruby has no YJIT |

Δ is cameo's time as a difference from the Ruby to its left: 🟢 under -10%,
🟡 within ±10%, 🔴 over +10%. The scripts read the clock themselves and
print `result seconds` last, so no process start is in a time, and YJIT's
times include its warm-up. The runner fails when the four runs print
different results.

Every script compiles at least one method; `tests/performance_test.go`
holds that, so that a change to the compiler cannot turn a table into the
interpreter timed twice.
