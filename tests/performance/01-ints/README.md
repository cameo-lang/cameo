# Recursion over Integers

Three of CRuby's benchmarks, each a top-level method over Integers that fit
64 bits, which `cameo build` compiles whole: the recursion runs as Go calls
on int64, and nothing returns to the interpreter.

| Script | What it runs |
|---|---|
| [fib.rb](fib.rb) | `fib(34)` by plain recursion, as CRuby's app_fib |
| [tak.rb](tak.rb) | `tak(18, 9, 0)`, as app_tak |
| [tarai.rb](tarai.rb) | `tarai(12, 6, 0)`, as app_tarai |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, cameo c3e80a7, CRuby 4.0.7, on
2026-10-21. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 01-ints/fib | 1.420 | 0.296 | 🔴 +379% | 0.024 | 0.048 | 🟢 -51% |
| 01-ints/tak | 2.702 | 0.462 | 🔴 +485% | 0.033 | 0.076 | 🟢 -57% |
| 01-ints/tarai | 2.113 | 0.308 | 🔴 +585% | 0.026 | 0.058 | 🟢 -55% |

Compiled, the three take about half of YJIT's time, and a sixtieth to an
eightieth of the interpreter's.
