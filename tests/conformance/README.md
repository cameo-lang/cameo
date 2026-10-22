# Compiled code against the interpreter

[run.rb](run.rb) runs every script cameo holds to an output twice, on the
interpreter and with the top-level methods `cameo build` compiles, and
holds the compiled run to the interpreter's output and exit status:

| Scripts | From |
|---|---|
| the examples | [examples/*.cameo](../../examples), each with its `.expected` |
| the site's samples | the `<pre class="cameo">` blocks of [doc/gh_pages](../../doc/gh_pages) |
| CRuby's benchmarks | the 852 cases of [benchmarks/ruby/works](../../benchmarks/ruby/works) that run under cameo with Ruby's result |

```sh
ruby tests/conformance/run.rb                    # every script, half the cores at once
ruby tests/conformance/run.rb --jobs 2 examples  # the examples alone
make conformance
```

Both runs go through a generated main that runs the script and prints its
last value after its output, `=> 5702887` for a script that ends in
`fib(34)`: most of CRuby's benchmarks print nothing, and would agree on
that alone. One binary serves every compiled run: `cameo build` writes a
package per script under `bin/_conformance`, the main imports them all, and
`go build` links them into a program that runs the script it is given with
that script's methods compiled; the interpreter finds them by the program's
digest. A script none of whose methods compile is counted and not run,
since it would run the interpreter twice. A script whose two interpreter
runs print differently, one that draws random numbers or reads the clock,
is nondeterministic and held to nothing.

The report counts, per source, the scripts, those with a compiled method,
the methods compiled, and the runs that printed the same, differed, were
nondeterministic or failed to build. It names every script that differed or
failed to build, with the first line that differs, and fails on either.

This is the check to run for a change to `src/aot`. It is not part of
`make check`, which needs no tool but Go, and it grows with what compiles:
with 61 of 920 scripts compiling a method it takes ten seconds.
