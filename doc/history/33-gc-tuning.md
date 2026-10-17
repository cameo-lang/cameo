# 33. The collector at GOGC=400 with a 1 GB soft limit

Steps 9 to 12 made the evaluator allocate less. What it still allocates,
frames, scopes, strings, collections, is collected by Go's garbage
collector, which by default runs when the heap has grown to twice what
survived the last collection
([GOGC=100](https://tip.golang.org/doc/gc-guide#GOGC)). For a short-lived
script with a small live heap, that means many collections that find
little to free.

## The setting

The `amber` binary sets GOGC to 400, a heap of five times what survives,
and a soft memory limit of 1 GB
([GOMEMLIMIT](https://tip.golang.org/doc/gc-guide#Memory_limit)), unless
the environment sets either ([src/cmd/amber/main.go:33](https://github.com/radiospiel/amber/blob/2b9dcdae8dbfe15a011ef35e369eb2f8036cef21/src/cmd/amber/main.go#L33-L40)).
Past the limit the collector runs as often as it must, so a script with a
large heap does not grow it fivefold. A Go program that embeds amber
through the `amber` package keeps its own settings: the tuning is the
binary's, not the library's.

## Measured

The setting was chosen by measurement: across CRuby's benchmark suite it
takes a quarter off the total time, and `5000!` runs in a third of the time,
while the median script's memory does not change. On the history
benchmarks of this repository, steps 32 and 33:

| | fib | block | hash | calls |
|---|---|---|---|---|
| step 32 | 0.33 s | 0.39 s | 0.054 s | 0.57 s |
| step 33 | 0.28 s | 0.32 s | 0.034 s | 0.49 s |

It matters most for `async`. Step 7 found four processes held back by the
collector; with the history's allocations removed and this setting, the
same four `fib(24)` calls ([07-par_seq.arb](07-par_seq.arb),
[07-par_async.arb](07-par_async.arb)) take:

| | in sequence | four processes |
|---|---|---|
| step 7 | 1.75 s | 1.35 s |
| step 32 | 0.35 s | 0.30 s |
| step 33 | 0.28 s | 0.13 s |

The processes now use the cores they are given.
