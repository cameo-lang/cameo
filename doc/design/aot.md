# Ahead-of-time compilation

`cameo build script.cameo dir` writes Go source for the script's top-level
methods that it can type without running the script. The host's own
`go build` compiles them into its binary, and the interpreter calls them in
place of the methods' bodies. Nothing is loaded at run time, and the binary
stays one static file on every platform Go supports.

This is step one, methods over Integers, the step that took them past 64
bits, the one that gave them blocks, the one that gave them Floats, and the
one that gave them classes. The interpreter still runs everything else in
the script, and the rest of this note keeps the order the steps would land
in.

## What compiles

A top-level method, or a method of a top-level class with typed fields,
compiles when:

- its parameters are required and positional, and each is an Integer, or
  an instance of the one class that declares a field the body reads on it;
- no other `def` in the program, in a class or anywhere else, has its name;
- its body types to an Integer or a Float from:
  - Integer and Float literals, `true` and `false`;
  - locals set at the method's top level before they are read, each of one
    type;
  - `+ - * / %`, unary minus and the six comparisons, on Integers, Floats
    and the two mixed, which gives a Float; `to_f` and `to_i`;
  - `&&`, `||`, `!`;
  - `if` and `unless` with both branches, `while`, `until` and `return`;
  - `times`, `upto` and `downto` on an Integer and `each` on a Range of
    Integers, with a block of one parameter at most, whose body is a
    statement over the same, with `break` and `next`;
  - reads of `int`, `float`, `bool` and `ref` fields, of `self` and of
    instances the method holds, and `self` itself;
  - calls of other compiled methods, directly, on `self` and on instances
    whose class is known.

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
`+ - *` checks for overflow, and division checks for zero. Every method has
a twin on `*big.Int`, `big_fact` beside `fn_fact`: an overflow ends the
`int64` run, and the twin runs the call from the start, with every call
below it on `*big.Int` too, and never returns to the interpreter. An
argument beyond 64 bits goes to the twin at once. Starting over repeats no
effect, because a compiled method does nothing but compute a value; the
twin allocates a `big.Int` per operation, so it is for the calls whose
numbers are big, and `int64` for the rest.

Floats are `float64` in both twins, and a method's result may be one. The
type of a method's value is settled by checking the candidates in rounds:
a call of a method not yet typed is unknown, which joins with anything, and
a method's type, once found, types its calls in the next round, until
nothing changes. A method that returns through itself alone gets Integer. A
local holds one type, and the branches of an `if` and a method's returns
must agree, so `n > 0 ? 1 : 1.5` is left to the interpreter. Integer
against Float compares exactly, through `object.CompareNumbers`, as the
interpreter does; `to_i` of NaN or an infinity is left to the interpreter,
which raises.

Float arithmetic is written as `float64(a * b)`: the explicit conversion
rounds the product, which keeps Go from fusing it with the sum that follows
into one instruction with one rounding, as it does on arm64. The
interpreter rounds every operation, and one point of a Mandelbrot count
moved before the conversion was there.

A class's method is registered as `Class#name` and installed on the class
by `evalDef`; it runs with the instance as `self`, a `*object.Instance`. A
field read is a load from `self.Fields` by the index the class fixes, the
parent's fields first, through a helper that checks the Value holds what
the field declares and leaves the call to the interpreter when it holds
nil. A parameter the body reads a field on is an instance of the class that
declares the field, when one class alone does; the entry checks the
argument's class by name, through its ancestors, so an instance of a child
class passes. A call on an instance of a known class, or a bare call in a
class's method, is a direct Go call when it reaches a compiled method of the
class or of a class above it. Construction and field assignment stay with
the interpreter: `initialize` is the interpreter's, and a field is set in
`initialize` alone.

An argument that is not an Integer and a division by zero return
`eval.ErrDeopt`. The interpreter then runs the method's body from the
start, and the calls below it run on the interpreter too, since each of them
would leave again. Division by zero raises, and the interpreter has the line
to raise it at.

A block of `times`, `upto`, `downto` or `each` compiles to a Go loop with
the body inside it, not to a closure: the bounds are read once, before the
loop; the counter is the loop's own, so that the body may set the parameter
without moving the loop; a local the body first sets is declared in the
body, new on every pass, as a block's locals are; `break` and `next` are
Go's, and `return` leaves the method. `times`, `upto` and `downto` have
their receiver as value; `each` returns its Range, so it compiles as a
statement only. On `int64`, a loop up to the largest Integer or down to
the smallest is left to the twin, where the counter cannot wrap.

A compiled method checks the run's stop every 4096 calls and loop passes,
so `RunContext` stops a run inside compiled code. A call counts on the
state it carries; a loop counts on its own variable, by its low bits, or on
a local counter where the variable is a `big.Int` or a `while` has none. A
store to the state on every pass cost 60% of a loop that adds two numbers.

## Measured

[tests/performance](../../tests/performance) times CRuby's benchmarks on
the interpreter, compiled and on CRuby, one directory per step, with the
run that landed the step in its README. On recursion over Integers the
compiled methods run about sixty times faster than the interpreter, ten
times faster than CRuby 4.0, and twice as fast as CRuby 4.0 with YJIT. On
factorial, whose product outgrows 64 bits after 20 steps, the twin on
`*big.Int` takes half the interpreter's time and a quarter of YJIT's. The
block loops are in [tests/performance/03-blocks](../../tests/performance/03-blocks),
and Float arithmetic in [04-floats](../../tests/performance/04-floats), where
a Mandelbrot grid and a Leibniz series compiled take a fifteenth of YJIT's
time. [05-classes](../../tests/performance/05-classes) has the Mandelbrot
grid in a class whose fields hold the grid, and a pair of vectors read
through `ref` fields, both a hundred times faster than the interpreter.

## Next steps

1. Construction in Go: `Class.new` for a class whose `initialize` sets
   fields from its parameters alone, so that a method may build the
   instances it works on, and a whole numeric loop over instances compiles.
2. Parameters typed from the program's call sites, so that a method may
   take a Float, or an instance it reads no field on.
3. Strings, and Arrays of a known element type, which use the runtime's
   own values where Go has no type of its own for them.
4. Blocks that compute a value, `map`, `sum` and `select` on a Range, and
   `loop` with `break` and its value.
5. Calls from compiled code into the interpreter for what stays dynamic,
   such as `send` and values read from JSON, so that one construct the
   compiler lacks leaves only itself to the interpreter.

[tests/conformance](../../tests/conformance) is the conformance suite: the
examples, the site's samples and the 852 CRuby benchmark cases, each run on
the interpreter and compiled, and the two held to the same output. Every
step runs it before it lands.
