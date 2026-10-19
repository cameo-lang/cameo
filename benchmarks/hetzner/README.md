# Benchmarks on a 16-core server

What it takes to run cameo's benchmarks on a temporary Hetzner server. The
skill [hetzner-benchmarks](../../.claude/skills/hetzner-benchmarks/SKILL.md)
is the procedure, from creating the server to deleting it.

- [provision.sh](provision.sh) prepares a fresh Ubuntu 24.04 server: Go at
  go.mod's version, and CRuby 3.4 and 4.0 built from source into
  `/opt/ruby-<version>`.
- [fib/run.sh](fib/run.sh) times Fibonacci on cameo binaries at 1, 4 and
  16 cores, and on every CRuby in `/opt`.
- [../compare](../compare) is the suite of 852 CRuby benchmarks.

## The fib scripts

Every script reads the clock inside the run, so no start is in a time, and
each time is the best of three processes.

| Script | What it runs | n |
|---|---|---|
| [seq.rb](fib/seq.rb) | plain recursion | 32 |
| [list.rb](fib/list.rb) | recursion that collects both parts in a list, as async.rb does without `async` | 32 |
| [coarse.rb](fib/coarse.rb) | `async` in the top ten levels, plain recursion below | 32 |
| [async.rb](fib/async.rb) | `async` at every level | 28 |
