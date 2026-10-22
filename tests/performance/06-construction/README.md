# Construction

Two scripts whose loops build instances: a walk where every step makes a
new vector, and two bodies rebuilt every step of a simulation, since a
body's fields are set in its construction alone. Compiled, `Class.new`
reads the arguments, puts the fields' values into a slice by index and
calls `eval.Construct`, which converts and checks each as `initialize`'s
assignment does. Ruby cannot run the scripts, since the field
declarations are cameo's, so the Ruby columns are "-".

| Script | What it runs |
|---|---|
| [vectors.rb](vectors.rb) | `walk(2_000_000)`: each step `v.plus(step.scaled(i % 3))`, two `Vec.new` per step, four million instances |
| [bodies.rb](bodies.rb) | `orbit(1_000_000)`: each step two `Body#pulled`, each a `Body.new` of five Float fields |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, on 2026-10-21, at the commit that
landed construction. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 06-construction/bodies | 3.462 | - | - | 0.180 | - | - |
| 06-construction/vectors | 2.216 | - | - | 0.231 | - | - |

Before this step neither method compiled, since `Vec.new` and `Body.new`
were the interpreter's, so the compiled column was the interpreter's. A
construction costs about 55 ns compiled: the instance, its fields and the
slice of values are three allocations, and each value is converted and
checked against its field; the arithmetic around it is a few nanoseconds.
Setting the fields straight into the instance is the next step of
`doc/design/aot.md`.
