# Ahead-of-time compilation

`cameo build script.cameo dir` writes Go source for the script's top-level
methods that it can type without running the script. The host's own
`go build` compiles them into its binary, and the interpreter calls them in
place of the methods' bodies. Nothing is loaded at run time, and the binary
stays one static file on every platform Go supports.

This is step one: methods over Integers. The interpreter still runs
everything else in the script, and the rest of this note keeps the order the
steps would land in.

## What step one compiles

A top-level method compiles when:

- its parameters are required and positional;
- no other `def` in the program, in a class or anywhere else, has its name;
- its body types to an Integer from:
  - Integer literals, `true` and `false`;
  - locals set at the method's top level before they are read;
  - `+ - * / %`, unary minus and the six comparisons;
  - `&&`, `||`, `!`;
  - `if` and `unless` with both branches, `while`, `until` and `return`;
  - calls of other compiled methods.

Everything else leaves the method to the interpreter, and `cameo build`
names the first construct that did:

```
$ cameo build examples/aot/arith.rb examples/aot/arith arith
compiled fib
...
interpreted collatz: line 57: it calls even? on Integer
interpreted greet: line 81: an interpolated String is not compiled yet
```

The second rule is what makes direct calls safe. A compiled `fib` calls
`fn_fib` in Go, which is right only if every call of `fib` reaches the one
definition. The classes are closed, so a script cannot redefine `Integer#+`,
and the compiler can call Integer's operators directly.

## cameo run

`cameo run script.cameo` writes what `cameo build` writes, without its
report, into a temporary module, builds it with the go command and runs the
program, which exits as the script does. The module requires cameo through
a `replace` to the source the cameo binary was built from, found through
the path Go recorded for it: a checkout for `go build`, the module cache
for `go install`. The generated code fits only the version of cameo that
wrote it, so no other source will do; a binary built with `-trimpath`
records no such path, and `cameo run` refuses to run. The module takes
cameo's `go.sum`, so the build downloads nothing that building cameo did
not, and Go's build cache makes a second run of the same script cost a
link.

## How a compiled method runs

The generated package registers its methods under the digest of the program
they were compiled from, `syntax.Digest` of the parsed script. When a run
evaluates a top-level `def` of that program, the method gets the Go
function as its `Native` and keeps its body. A different script, or the
same one edited, has another digest and runs on the interpreter alone.

Integers are arbitrary precision, and compiled code runs on `int64`. Each
`+ - *` checks for overflow, and division checks for zero. An argument that
is not a 64-bit Integer, a result past 64 bits and a division by zero all
return `eval.ErrDeopt`. The interpreter then runs the method's body from the
start, and the calls below it run on the interpreter too, since each of them
would leave again. Starting over repeats no effect, because a compiled
method does nothing but compute a value.

A compiled method checks the run's stop every 4096 calls and loop passes,
so `RunContext` stops a run inside compiled code.

## Measured

[benchmarks/aot](../../benchmarks/aot) times four of CRuby's benchmarks on
the interpreter, compiled and on CRuby. On recursion over Integers the
compiled methods run about 100 times faster than the interpreter, ten times
faster than CRuby 3.4 and 4.0, and 1.7 times faster than either with YJIT.
Factorial gains nothing: its product outgrows 64 bits after 20 steps, so
every call goes back to the interpreter.

## Next steps

1. Floats, Strings, and Arrays of a known element type, which use the
   runtime's own values where Go has no type of its own for them.
2. Methods of classes with typed fields: a field read becomes a struct
   field, and a call on a receiver whose class is known becomes a direct
   call.
3. Bignums in compiled code: a checked operation falls back to `big.Int`
   in place, and stops returning to the interpreter.
4. Blocks as Go closures, which brings `each`, `map` and `times`.
5. Calls from compiled code into the interpreter for what stays dynamic,
   such as `send` and values read from JSON, so that one construct the
   compiler lacks leaves only itself to the interpreter.

The examples with their expected output, the site's samples and the 852
CRuby benchmark cases become the conformance suite. Each case runs on both
the interpreter and the compiled build.
