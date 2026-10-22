# Blocks as loops

Two methods whose work is a block: `Integer#times` over a running sum, and
`Range#each` three deep. `cameo build` compiles each block call to a Go
loop with the body inside it, so the compiled build calls no block at all.

| Script | What it runs |
|---|---|
| [times.rb](times.rb) | the sum of the Integers below 30 million, `n.times { \|i\| s += i }`, as CRuby's loop_times with a body |
| [triples.rb](triples.rb) | the Pythagorean triples with sides up to 400, counted by `(1..n).each`, `(a..n).each` and `(b..n).each` inside one another |

## Results

Mac mini (Apple M1, 8 cores), Go 1.26.4, CRuby 4.0.7, on 2026-10-21, at
the commit that landed the block loops. Seconds, best of three:

| script | cameo interp | ruby 4.0.7 (100%) | Δ | cameo compiled | ruby 4.0.7 yjit (100%) | Δ |
|---|---|---|---|---|---|---|
| 03-blocks/times | 3.209 | 1.199 | 🔴 +168% | 0.062 | 0.963 | 🟢 -94% |
| 03-blocks/triples | 1.688 | 0.563 | 🔴 +200% | 0.034 | 0.278 | 🟢 -88% |

YJIT calls the block thirty million times; the compiled build runs a loop
of two checked additions and a stop check, fifty times faster than the
interpreter. Before this step, both scripts ran on the interpreter whole,
so the compiled column was the interpreter's.
