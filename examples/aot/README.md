# Compiled examples

Scripts whose top-level methods `cameo build` compiles to Go, each with the
package it generated. The test runs every script compiled and on the
interpreter alone, holds both to `<name>.expected`, and fails when a
generated package differs from what `cameo build` writes now.

| Script | What it shows |
|---|---|
| [arith.rb](arith.rb) | Integer arithmetic, loops and early returns, a result past 64 bits and an argument beyond them, which the twin on `*big.Int` computes, and the calls the compiled code leaves to the interpreter: a Float argument, a division by zero |
| [blocks.rb](blocks.rb) | `times`, `upto`, `downto` and `each` over a Range as Go loops, with `break`, `next`, `return`, a local of the block's own, a parameter the body sets, and bounds beyond 64 bits |
| [floats.rb](floats.rb) | Float literals and locals, arithmetic with Integers, exact comparisons of the two, `to_f` and `to_i`, a Float result found through recursion, and Integers past 64 bits beside Floats |
| [spin.rb](spin.rb) | a compiled loop that never ends, which a host's `RunContext` stops |

To regenerate one:

```sh
bin/cameo build examples/aot/arith.rb examples/aot/arith arith
```
