# Pause and resume

`cameo script.cameo` pauses on `^C`, writes the run to a temporary file, and
exits with status 3. `cameo resume <dump>` continues the run; `cameo inspect
<dump>` shows what the dump holds. `cameo --no-pause script.cameo` keeps
`^C` ending the script, as it ends any other process. From Go,
`Program.Pause`, `Program.Dump`, `cameo.Resume` and `cameo.InspectDump` do
the same.

## Where a run pauses

A run pauses before an instruction: inside a method, some calls deep,
inside a `while` loop, a block a method yields to, a `rescue` or an
`ensure` clause, as well as between two top-level statements. `^C` sets a
flag, and the evaluator reads it before each instruction. A script whose
one statement is a loop that never ends pauses in the loop.

What it takes is that the run's state is data. The evaluator keeps a run
as a stack of activations, one per method or block that is running, each
with its position, its value stack, its scope and its frame
([ir.md](ir.md#the-activation-stack)), and the dump holds that stack. A
builtin that calls back into the script, `list.each` its block or `puts`
a `to_s`, is Go code in the middle of its work, which no dump holds: the
evaluator runs such a call in a loop of its own, and a run pauses only
while no such loop is running. So a pause that arrives inside the block
of `each` takes effect when `each` returns. The builtins a script spends
its time in become step functions, whose state is data too, group by
group; until one has, it only delays a pause.

The builtins that are step functions so far, inside whose block a run
pauses at once: `Integer#times`, `upto` and `downto`; `loop`; `each` and
`each_index` of a List; and, of a List or a Range, `each_with_index`,
`each_with_object`, `map`, `flat_map`, `filter_map`, `select`, `reject`,
`partition` and `find`, under each of their names. The dump
holds such a builtin as an activation of its own, between the call that
made it and its block: the method, the call it was made with, and what it
keeps from one call of the block to the next, for `times` the number it
came to. A dump that is inside a builtin this cameo has no step function
for is refused.

The same holds for these, each for as long as it takes: a class body, an
`initialize` that `new` called, a parameter's default, `super`, an
operator a class defines, `Proc#call`, a `require` of a library written in
cameo, and a call the host made into the program.

The pause waits for every call `async` started, so their results travel
settled in the dump. A promise the script or a host made by hand and has not
settled cannot wait and is left out. A script that is waiting in `await`
pauses when the promise settles.

## What a dump holds

The dump is the protobuf message `cameo.Dump`, written in
[src/cameopb/cameo/dump.proto](../../src/cameopb/cameo/dump.proto):

| field | holds |
|---|---|
| `file`, `source` | the script's path and text |
| `next`, `line` | the index of the top-level statement the run continues with or in, and the line of its next instruction |
| `stack`, `scopes`, `frames` | for a run that paused inside a statement: its activations, and the scopes and frames they run in |
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

A symbol in the dump is a symbol after the resume. Everywhere else a
value crosses the envelope a symbol reads back as a String
([cameo.md](cameo.md#symbols)), the one form every reader has; a dump is
read by the run it was, which compares against `:name` and looks keys up
in `{name: …}`. So the dump writes a symbol as `symbol_value`, which no
other message does.

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

An instance in the dump names its class and the class's digest. The dump
carries the class of every instance it holds, so an instance of a class the
script reopened after it made the instance comes back as an instance of the
class it was made of, not of the one the name is bound to now. A closure
names the classes of its binding by name alone, and comes back with the
ones the names are bound to.

A dump resumes on a cameo of its IR version: `version` says which, and a
cameo of another version refuses the dump, as it refuses one written before
the dump carried code. The dump still holds the script's text, which the
resumed run parses for the statements after the pause point.

## How a resumed run gets its activations back

A run that paused inside a statement has its activations in `stack`, the
outermost first: the statement, then each call in flight.

| of an activation | the dump holds |
|---|---|
| its code | the proc of a unit, as a method names its code; the statement is `next` |
| its position | a cursor per list it is inside: the index of the next instruction, which list of its instruction it is, and the phase the instruction is in |
| its value stack | the operands of the expressions in flight, as the variables are held |
| its scope and frame | an index into `scopes` and into `frames` |

A `begin` that is in a rescue clause keeps the exception on its cursor, and
one whose ensure clause runs keeps what the clause interrupted, a `return`
with its value or an exception, so that it goes on when the clause ends.

Scopes and frames are tables because they are shared. A block's activation
runs in the frame of the method the block was written in: that is how its
`return` finds the method and its `yield` the method's block. So a frame is
in the dump once, and every activation of it has its index. A Proc that is
a value, in a variable or on a value stack, names the frame it was written
in the same way while that frame's call is running; once the call has
ended it carries its binding, as a Proc does in any other message. Values
need no table: they never change, so a copy is the value.

A value in flight that the envelope cannot carry is nil after the resume,
and a local is unbound; `missing` names each when the run pauses.

Some of what an activation holds it cannot go on without: the block of a
builtin that is between two calls of it, the exception a rescue clause
is handling. When one of those cannot travel there is no dump, and
`Dump` says why. The run is not lost: `Run` on a program that paused goes
on where it paused, between two statements or inside one. `cameo` does
that when it cannot dump a script or cannot write the file: it says so
and the script goes on, to be paused again by the next `^C`.

A resumed run finds every activation's proc by unit and index and walks
the cursors down its lists. A dump whose path does not fit the code, a
cursor past the end of its list, is refused.
