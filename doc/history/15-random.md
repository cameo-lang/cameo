# 15. Random numbers from a seedable generator

102 cases of the benchmark suite stopped on `rand`, `srand`, `shuffle` or
`sample`. Ruby draws from a Mersenne Twister with a process-wide state,
which `srand` reseeds. amber has no process-wide state to change, and two
questions had to be decided: where the generator lives, and whose numbers
it draws.

## One generator per interpreter

Each interpreter holds a generator from Go's
[`math/rand/v2`](https://pkg.go.dev/math/rand/v2), a
[PCG](https://www.pcg-random.org/), seeded at random when the interpreter
starts ([src/eval/interp.go:70](https://github.com/radiospiel/amber/blob/26a86ba5e3613907ff0441c19df7da6d6def3a75/src/eval/interp.go#L70-L75)). `srand(n)`
replaces it with one seeded from `n` and returns the previous seed, as
Ruby's does. An `async` worker's generator is seeded from its parent's
when the process starts, so a seeded script draws the same numbers on
every run, its processes included.

The generator is the one piece of state in amber that changes when it is
used. It is the interpreter's, not a value's, so nothing a script can hold
changes, and two processes never share one.

## Whose numbers

Reproducing Ruby's sequence would mean porting MT19937 and CRuby's
conversions from its output to floats and ranges, so that `srand(1); rand`
printed what Ruby prints. That was decided against as not worth its code:
a script that seeds the generator gets the same numbers on every run of
amber, but not Ruby's. The design records the decision.

`Kernel#rand` follows Ruby's semantics for its argument (none, an
integer, a float, a range), and `Array#shuffle` and `#sample` draw from
the same generator; `sample(n)` is a partial shuffle that costs the
sample's size, not the array's.

## The benchmarks

106 cases move to `works/`, held to what holds for any draw rather than
to Ruby's numbers; six reach `Complex` and move to `unsupported/complex`.
The benchmark runner's default timeout rises to 300 s, since one working
case takes 68 s.
