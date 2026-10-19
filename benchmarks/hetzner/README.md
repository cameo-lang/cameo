# Benchmarks on a 16-core server

What it takes to run cameo's benchmarks on a temporary Hetzner server, and
what the last run measured. The skill
[hetzner-benchmarks](../../.claude/skills/hetzner-benchmarks/SKILL.md) is
the procedure, from creating the server to deleting it.

- [provision.sh](provision.sh) prepares a fresh Ubuntu 24.04 server: Go at
  go.mod's version, Rust, and CRuby 3.4 and 4.0 built from source into
  `/opt/ruby-<version>`.
- [fib/run.sh](fib/run.sh) times Fibonacci on cameo binaries at 1, 4 and
  16 cores, and on every CRuby in `/opt` without a JIT, with YJIT and, on
  4.0, with ZJIT. [fib/results.tsv](fib/results.tsv) has the last run.
- [../compare](../compare) is the suite of 852 CRuby benchmarks; its
  README has the last run's summary.

## The fib scripts

Every script reads the clock inside the run, so no start is in a time, and
each time is the best of three processes.

| Script | What it runs | n |
|---|---|---|
| [seq.rb](fib/seq.rb) | plain recursion | 32 |
| [list.rb](fib/list.rb) | recursion that collects both parts in a list, as fib.rb's sequential version | 32 |
| [coarse.rb](fib/coarse.rb) | `async` in the top ten levels, plain recursion below | 32 |
| [async.rb](fib/async.rb) | `async` at every level | 28 |

## Results

On 2026-10-19, on a Hetzner ccx43: 16 dedicated AMD EPYC-Milan vCPUs.
`main` is 6ffe953; `queue` is the commit that queues async calls for a few
runners, with the changes below it.

Seconds, with the peak RSS of `async.rb`:

| | cores | seq | list | coarse | async |
|---|---|---|---|---|---|
| cameo main | 1 | 1.45 | 3.63 | 1.46 | 17.18 (1025 MB) |
| cameo main | 4 | 1.09 | 3.02 | 0.61 | 22.45 (1740 MB) |
| cameo main | 16 | 1.30 | 3.30 | 0.42 | 24.43 (1845 MB) |
| cameo queue | 1 | 1.46 | 3.72 | 1.50 | 1.48 (77 MB) |
| cameo queue | 4 | 1.14 | 3.13 | 0.60 | 0.62 (42 MB) |
| cameo queue | 16 | 1.28 | 3.30 | 0.46 | 0.42 (116 MB) |
| CRuby 3.4.11 | 1 | 0.155 | 0.318 | | |
| CRuby 3.4.11 YJIT | 1 | 0.021 | 0.129 | | |
| CRuby 4.0.7 | 1 | 0.144 | 0.326 | | |
| CRuby 4.0.7 YJIT | 1 | 0.021 | 0.120 | | |
| CRuby 4.0.7 ZJIT | 1 | 0.139 | 0.242 | | |

- Fine-grained `async` went from slower on more cores to faster: 17.2 s on
  one core and 24.4 s on 16, to 1.48 s and 0.42 s, with 77–116 MB of
  memory instead of 1.0–1.8 GB.
- Coarse `async` takes a third of the sequential time on 16 cores, and
  stops gaining past about four: most of the remaining time is allocation,
  which Go's allocator does not spread over cores.
- The sequential scripts did not change. cameo runs them nine to eleven
  times slower than CRuby without a JIT. YJIT runs plain recursion seven
  times faster than CRuby without a JIT; ZJIT, 10% faster.
- The sequential scripts run faster on four cores than on one: the garbage
  collector works on the other cores.
