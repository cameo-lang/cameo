# The debugger and irb

`cameo debug script.cameo` runs a script under the debugger, stopped before
its first statement. A script run with `cameo script.cameo` that requires
`"debug"` stops at `debugger`, `binding.break` and `binding.b`, as a Ruby
script with the debug library does. Without the library those three are
undefined, as in Ruby. From Go, `Program.Debug` switches the debugger on and
`Program.Console` names where a debugger the script switches on talks;
without a console, its stops do nothing.

A stop shows the source around the line about to run, and the frame, as
rdbg does:

```
[1, 10] in fib.cameo
=>    1| def fib(n)
      2|   return n if n < 2
      ...
=>#0	<main> at fib.cameo:1
(rdbg)
```

## Commands

| command | does |
|---|---|
| `step`, `s` | runs to the next statement, into calls |
| `next`, `n` | runs to the next statement in this frame, over calls, class bodies and blocks |
| `finish`, `fin` | runs until this frame returns, and stops at the caller's next statement |
| `continue`, `c` | runs until a breakpoint, `debugger` or `binding.break` |
| `break LINE`, `b LINE` | stops before line LINE, each time it runs; `b file:LINE` is the same |
| `break METHOD` | stops at the first line of METHOD: `fib`, `Point#sum`, `Point.new_from` |
| `break` | lists the breakpoints |
| `delete N`, `del N` | deletes breakpoint N; `delete` alone deletes all |
| `backtrace`, `bt` | the frames, innermost first, each with its arguments: `fib(n=3)` |
| `info`, `i` | `%self` and the locals of the frame |
| `list`, `l` | the source around the current line; `list LINE` around LINE |
| `up`, `u`; `down`, `d` | select the outer or the inner frame, for `info`, `p` and `list` |
| `frame N`, `f N` | select frame #N |
| `p EXPR`, `pp EXPR` | evaluate EXPR in the selected frame and print its value; any other input is an EXPR |
| `quit`, `q` | end the run; `cameo debug` then exits 0 |
| `help`, `h` | the commands |

A stop lands before a statement, so `step` and `next` move a statement at
a time, not an expression. The end of the console continues the run
without further stops.

## What it does not do

- Workers `async` starts have no debugger: a breakpoint in a method a
  worker runs does not stop.
- The debugger sees the statements compiled after it was switched on.
  `cameo debug` switches it on before the script runs; `require "debug"`
  in the middle of a script misses the bodies that ran before it.
- `step` into a builtin method stops at the next statement of the script,
  since a builtin has no lines.
- `cameo irb` has no debugger: `debugger` and `binding.break` typed in,
  or in the script it runs first, stop nothing, since the lines typed in
  are the console.

## irb

`cameo irb` reads lines and evaluates each at the top level, printing its
value as `p` would; `cameo irb script.cameo` runs the script first, so the
lines see its methods, classes and variables. A line that is not a whole
expression yet, `def f` or `[1,`, waits for the next lines. An exception
prints its message and the loop goes on. `exit`, `quit` or the end of the
input ends it.

When its input and output are a terminal, `cameo irb` puts the terminal in
raw mode and edits the line itself, with golang.org/x/term: the arrow keys,
Ctrl-A, Ctrl-E, Ctrl-K and Ctrl-W, and a history on the up and down arrows.
Ctrl-C drops the line being typed, and an expression that is not whole yet;
Ctrl-D on an empty line ends the loop.

Tab completes the name before the cursor: to the one name it can be, or to
the longest start the names share, and lists them when the name is that start
already. The names are the variables, the methods of the top level, the
constants and the keywords; after a dot they are the methods of the receiver,
when it is a variable, a constant or a literal. Completion runs no method: a
receiver that is a method call, `items.first.`, completes nothing. Piped input is read a line at a time,
as before. The loop is the package `src/irb`: a `Session` evaluates lines,
and a `Terminal` edits them on any byte stream, so a program that embeds
cameo, such as an app's terminal without a tty, runs the same loop.

The parser reads a bare name as a variable only once it saw it assigned,
so each line is parsed behind a prelude that names the variables bound so
far, `x = nil if false;`, which never runs. The debugger's `p` does the same
with the frame's locals.
