# Compiled benchmarks

Four of CRuby's benchmarks, timed on the interpreter, compiled with
`cameo build`, and on CRuby. [run.sh](run.sh) builds each script, runs
every case three times, and prints the best in-script seconds; the scripts
read the clock inside the run, so no start is in a time. See
[doc/design/aot.md](../../doc/design/aot.md) for what the compiler takes.

```sh
benchmarks/aot/run.sh                       # ruby on PATH
RUBIES="ruby /opt/ruby-3.4/bin/ruby" benchmarks/aot/run.sh
```

| Script | What it runs |
|---|---|
| [fib.rb](fib.rb) | `fib(34)` by plain recursion |
| [tak.rb](tak.rb) | `tak(18, 9, 0)` |
| [tarai.rb](tarai.rb) | `tarai(12, 6, 0)` |
| [fact.rb](fact.rb) | `fact(5000)` a hundred times: past 20! every compiled call goes back to the interpreter |

## Results

On 2026-10-21, in a cloud container with 4 cores of an Intel Xeon at
2.8 GHz, with Go 1.26.4 and this branch at 73abd87. Both CRubies were built
from their git tags with `--enable-yjit`. Each YJIT time includes the JIT's
warm-up inside the run. Seconds:

| | fib | tak | tarai | fact |
|---|---|---|---|---|
| cameo, interpreted | 3.611 | 6.009 | 4.996 | 1.453 |
| cameo, compiled | **0.031** | **0.041** | **0.032** | 1.390 |
| CRuby 3.4.11 | 0.338 | 0.499 | 0.400 | 0.893 |
| CRuby 3.4.11, YJIT | 0.052 | 0.073 | 0.059 | **0.739** |
| CRuby 4.0.7 | 0.322 | 0.473 | 0.376 | 0.903 |
| CRuby 4.0.7, YJIT | 0.052 | 0.075 | 0.059 | 0.918 |

On the three recursion benchmarks, compiled code takes 0.55 to 0.6 of
YJIT's time. On fact, YJIT on 3.4 is twice as fast as cameo either way. The
product outgrows 64 bits, so the work is bignum multiplication, which CRuby
does faster than Go's `math/big` as cameo uses it.
