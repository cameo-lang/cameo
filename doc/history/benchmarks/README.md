# The history benchmarks

Six small scripts the history documents measure each step with. Each runs
long enough for the interpreter's work to dominate its start:

| Script | What it stresses |
|---|---|
| [fib.arb](fib.arb) | method calls: a recursive `fib(27)` |
| [loop.arb](loop.arb) | integer arithmetic in a `while` loop, 3 M iterations |
| [block.arb](block.arb) | a block over a range of a million |
| [hash.arb](hash.arb) | a hash built key by key with `h[k] = v`, 20 000 keys |
| [calls.arb](calls.arb) | instances: `Point#+` on typed fields, 300 000 times |
| [strings.arb](strings.arb) | interpolation and string methods, 200 000 times |

`ruby run.rb bin/amber ruby` prints the best of three wall-clock times for
each script on each command. `calls.arb` declares typed fields, so MRI
cannot run it.
