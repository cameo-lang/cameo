# Float arithmetic

Two methods with Integer parameters whose work is on Floats: the inner loop
of a Mandelbrot count, and a series sum. Compiled, a Float is a `float64`
and its arithmetic is Go's; an Integer beside it converts once.

| Script | What it runs |
|---|---|
| [mandelbrot.rb](mandelbrot.rb) | the points of a 500 by 500 grid that stay bounded for 50 steps of z = z² + c, as CRuby's app_mandelbrot without Complex |
| [pi.rb](pi.rb) | pi by the Leibniz series, ten million terms |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, CRuby 4.0.7, on 2026-10-21, at
the commit that landed Floats. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 04-floats/mandelbrot | 2.517 | 0.723 | 🔴 +248% | 0.019 | 0.272 | 🟢 -93% |
| 04-floats/pi | 1.960 | 0.829 | 🔴 +137% | 0.023 | 0.434 | 🟢 -95% |

Before this step, a method with a Float literal in it ran on the
interpreter whole, so the compiled column was the interpreter's. The
Mandelbrot script declares `t = 0.0` before its `while`: a local first set
inside a loop is nil on the paths that skip the loop, and the compiler
leaves such a method to the interpreter.
