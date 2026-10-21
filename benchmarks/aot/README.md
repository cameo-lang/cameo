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
2.8 GHz, Go 1.26.4, cameo 7636c80 with this branch. Seconds:

| | fib | tak | tarai | fact |
|---|---|---|---|---|
| interpreted | 3.794 | 7.013 | 5.316 | 1.471 |
| compiled | 0.037 | 0.040 | 0.033 | 1.413 |
| CRuby 3.3.6, no YJIT | 0.346 | 0.499 | 0.374 | 0.853 |
