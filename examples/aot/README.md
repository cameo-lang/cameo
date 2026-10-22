# Compiled examples

Scripts whose top-level methods `cameo build` compiles to Go, each with the
package it generated. The test runs every script compiled and on the
interpreter alone, holds both to `<name>.expected`, and fails when a
generated package differs from what `cameo build` writes now.

| Script | What it shows |
|---|---|
| [arith.rb](arith.rb) | Integer arithmetic, loops and early returns, and the calls the compiled code leaves to the interpreter: a result past 64 bits, a Float argument, a division by zero |
| [spin.rb](spin.rb) | a compiled loop that never ends, which a host's `RunContext` stops |

To regenerate one:

```sh
bin/cameo build examples/aot/arith.rb examples/aot/arith arith
```
