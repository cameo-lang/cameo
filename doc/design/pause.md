# Pause and resume

`cameo script.cameo` pauses on `^C`, writes the run to a temporary file, and
exits with status 3. `cameo resume <dump>` continues the run; `cameo inspect
<dump>` shows what the dump holds. `cameo --no-pause script.cameo` keeps
`^C` ending the script, as it ends any other process. From Go,
`Program.Pause`, `Program.Dump`, `cameo.Resume` and `cameo.InspectDump` do
the same.

## Where a run pauses

A run pauses between two top-level statements, never inside one. The
evaluator compiles the script into Go closures and runs them recursively, so
the state of a statement that is running — the loop it is in, the method it
called, the block it is evaluating — lives on the Go stack, which nothing
can serialise. `^C` sets a flag; the interpreter reads it before each
top-level statement and pauses there. A top-level statement that never
ends, `loop do … end` or a call that does all the work, is never paused: a
second `^C` ends the process.

The pause waits for every call `async` started, so their results travel
settled in the dump. A promise the script or a host made by hand and has not
settled cannot wait and is left out.

## What a dump holds

The dump is the protobuf message `cameo.Dump`, built in
[src/protogen/dump.go](../../src/protogen/dump.go):

| field | holds |
|---|---|
| `file`, `source` | the script's path and text |
| `next`, `line` | the index of the top-level statement the run continues with, and its line |
| `variables` | the top-level variables, each as its name and its value in the envelope wire form (`cameo.Value`, see [cameo.md](cameo.md#protobuf)) |
| `constants` | the constants the script assigned, the same way; a class is named instead |
| `promises` | every promise, settled: its id, the method `async` ran, and its result or the exception it failed with |
| `loaded` | the libraries `require` loaded |
| `rand` | the state of the random number generator, so a seeded script draws on as if it had not paused |

A value the envelope cannot carry — a Proc, an Enumerator, a Hash with a
default proc — has no value in the dump; its binding says why instead.
`cameo` names each when it pauses, `cameo inspect` lists them, and a
resumed run does not have them: reading such a variable raises `NameError`.

## How a resumed run gets its classes back

Classes, modules and methods are not in the dump: they come from the script,
which the dump carries. `cameo resume` runs the script's definitions before
the pause point again — every top-level `class`, `module`, `def`, `include`
and `require` statement, in order, and nothing else — then restores the
variables, the constants and the promises, and runs the statements from the
pause point on. The output of a class body that prints is printed again; a
method a top-level statement other than a definition made, through
`define_method` in an `if` for instance, is not back.

The dump holds the script's text, so the resumed run is the script as it was
when it paused, whatever the file holds now. An instance in the dump names
its class and the class's digest; a definition that changed between the
pause and the resume would not match, but the text is the dump's own.
