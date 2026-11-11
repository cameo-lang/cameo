# Pause and resume

`cameo script.cameo` pauses on `^C`, writes the run to a temporary file, and
exits with status 3. `cameo resume <dump>` continues the run; `cameo inspect
<dump>` shows what the dump holds. `cameo --no-pause script.cameo` keeps
`^C` ending the script, as it ends any other process. From Go,
`Program.Pause`, `Program.Dump`, `cameo.Resume` and `cameo.InspectDump` do
the same.

## Where a run pauses

A run pauses between two top-level statements, never inside one. The
evaluator runs a statement's instructions recursively, so
the state of a statement that is running — the loop it is in, the method it
called, the block it is evaluating — lives on the Go stack, which nothing
can serialise. `^C` sets a flag; the interpreter reads it before each
top-level statement and pauses there. A top-level statement that never
ends, `loop do … end` or a call that does all the work, is never paused: a
second `^C` ends the process.

[ir.md](ir.md) is the plan that lifts this limit: an IR that protobuf
carries, and an evaluator that keeps a run's state off the Go stack.

The pause waits for every call `async` started, so their results travel
settled in the dump. A promise the script or a host made by hand and has not
settled cannot wait and is left out.

## What a dump holds

The dump is the protobuf message `cameo.Dump`, written in
[src/cameopb/cameo/dump.proto](../../src/cameopb/cameo/dump.proto):

| field | holds |
|---|---|
| `file`, `source` | the script's path and text |
| `next`, `line` | the index of the top-level statement the run continues with, and its line |
| `variables` | the top-level variables, each as its name and its value in the envelope (`cameo.Value`, see [cameo.md](cameo.md#protobuf)); a variable bound to nil has a value too, an empty one |
| `constants` | the top-level constants, the same way; one that is a class refers to the class instead |
| `version` | the IR version of the dump's code |
| `units` | the code of the run: the script's unit, and the unit of every library written in cameo it required |
| `classes`, `top` | the classes and modules the script defined, and the methods of the top level |
| `cameo_top` | whether the script included `Cameo` at the top level |
| `promises` | every promise, settled: its id, the method `async` ran, and its result or the exception it failed with |
| `loaded` | the libraries `require` loaded |
| `rand` | the state of the random number generator, so a seeded script draws on as if it had not paused |
| `at_exit` | the blocks `at_exit` registered, in order, which a resumed run runs when it ends |

A Proc is in the dump as a closure: the proc of a unit, and the values it
captured ([ir.md](ir.md)). A resumed run calls it as the run that paused
would have.

A value the envelope cannot carry — an Enumerator, a Proc that captured
one — has no value in the dump; its binding says why instead. `cameo`
names each when it pauses, `cameo inspect` lists them, and a resumed run
does not have them: reading such a variable raises `NameError`, and such
an `at_exit` block does not run. The same holds for a constant or an
attribute of a class: `missing` names it.

A symbol in the dump reads back as a String, as a symbol does wherever a
value crosses the envelope: a resumed run that compares a variable with
`:name` compares a String.

## How a resumed run gets its classes back

Classes, modules and methods are in the dump, as data. `cameo resume` runs
nothing of the script before the pause point: it makes the classes the dump
describes, then restores the variables, the constants and the promises, and
runs the statements from the pause point on. A class body that prints does
not print again, and a method that a statement other than a definition
made, a `def` in an `if` for instance, is back like any other.

A class in the dump is what the run had made of it:

| of a class | the dump holds |
|---|---|
| its place | its qualified name, the module it is defined in, its superclass, its digest |
| modules | the ones it includes and extends, in order |
| fields | name, number, type and default of each, and the class that declared it; the reserved numbers and names |
| methods | each as the proc of a unit, with the modules around its definition; a name `alias` gave a method Go implements, as that name and the method's |
| constants | the constants of its body, as the top level's |
| attributes | their values, and the defaults of the ones it declared |

A method's code is not in the class: a method names a proc of a unit by the
unit's digest and the proc's index, as a closure does, and `units` holds
every unit once. A resumed run compiles the script again, which gives the
same unit, and takes every other unit from the dump; a unit whose
instructions do not hash to its digest is refused.

A class refers to another by its qualified name. That is not enough for
every class a run holds: a `class` statement on a class that is there makes
a new class and binds it to the name, and what was made of the class before
keeps the one it had — a subclass its superclass, a class the module it
included. So the dump carries every class something of the dump refers to,
and a class its name is no longer bound to has its digest in its id,
`Base@8d03a7c54a9d2803`. A class Go defined or the host registered is the
process's and is not in the dump; a reference names it.

A library written in cameo is the script's code once it is required: its
classes and methods are in the dump, and its unit. Where it added to a
module Go defined, the dump holds the copy the run kept of that module, as
what it has beyond the module Go defined.

What a class holds that only Go knows is not in the dump: a resumed run
includes `Singleton` in a class again, which gives the class its methods
back, and not the instance it had made.

An instance in the dump names its class and the class's digest, and finds
the class by its name. An instance of a class the script reopened after it
made the instance has a class no name leads to, and a dump that holds one
does not resume.

A dump resumes on a cameo of its IR version: `version` says which, and a
cameo of another version refuses the dump, as it refuses one written before
the dump carried code. The dump still holds the script's text, which the
resumed run parses for the statements after the pause point.
