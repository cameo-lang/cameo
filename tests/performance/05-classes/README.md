# Methods of classes with typed fields

Two scripts whose work is in methods of classes, reading typed fields: the
Mandelbrot count of [04-floats](../04-floats) with the grid in `int` and
`float` fields, and a pair of vectors held through `ref` fields whose
methods read theirs. Ruby cannot run them, since the field declarations
are cameo's, so the Ruby columns are "-".

| Script | What it runs |
|---|---|
| [mandelbrot.rb](mandelbrot.rb) | `Grid#count`: the 500 by 500 grid of 04-floats, with the size, the step limit, the corner and the scale read from fields inside the loops |
| [pairs.rb](pairs.rb) | `Pair#churn`: ten million passes, each reading two `Vec`s from `ref` fields and calling `Vec#dot` and `Vec#dist2`, which read the vectors' fields |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, CRuby 4.0.7, on 2026-10-21, at
the commit that landed class methods. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 05-classes/mandelbrot | 2.839 | - | - | 0.022 | - | - |
| 05-classes/pairs | 10.050 | - | - | 0.110 | - | - |

The same count with the grid in locals, `04-floats/mandelbrot`, takes
0.019 s compiled, 0.272 s on CRuby 4.0.7 with YJIT and 0.723 s without: the
field reads inside the loops cost a sixth. In `pairs`, a pass reads two
`ref` fields and, in the two calls, eight `float` fields, each a checked
load; the interpreter dispatches the same as method calls, a hundred times
slower.
