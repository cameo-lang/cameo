# Exceptions as messages

Every exception is a protobuf message, as every instance is, and a script's
exception class declares typed fields like any class. An error that carries
data, an HTTP status or the URL that failed, reaches `await` from a worker
and reaches the Go host with that data, which is where errors with data
matter most.

## Context

An exception was a Go struct with a class, a message, a line and, for some
of Ruby's exceptions, a map of attributes (a KeyError's key). It crossed to
an awaiting interpreter as a shared Go pointer and could not travel in the
envelope at all. A script's exception class could declare fields, but its
`initialize` never ran: `E.new(404)` made the message "404", and reading a
field panicked the interpreter.

## Decisions

**An exception stays a Go struct and becomes a message when one is
needed.** `object.Exception` keeps its message, line and attributes, and
gains the cause and the fields its class declares, in the order the class
declares them, as an `Instance` holds them. The message is built when an
exception crosses to a worker's awaiter, goes to the host, or meets
`to_proto` or the envelope. `raise` and `rescue` never touch protobuf, which
keeps a raise in a loop close to its old cost (see *Performance*).

**The parts of an exception's message.** An exception class's message is
its parts, then the fields it declares:

| part | number | type | in |
|---|---|---|---|
| `message` | 1 | `string` | every exception |
| `line` | 2 | `int64` | every exception |
| `cause` | 3 | `cameo.Value` | every exception |
| `name` | 4 | `string`, read back as a Symbol | NameError |
| `receiver` | 5 | `cameo.Value` | NameError, KeyError |
| `args` | 6 | `repeated cameo.Value` | NoMethodError |
| `key` | 4 | `cameo.Value` | KeyError |
| `status` | 4 | `int64` | SystemExit |
| `tag`, `value` | 4, 5 | `cameo.Value` | UncaughtThrowError |

A subclass's message holds its superclass's parts, as a subclass's message
holds its parent's fields. Numbers 1 to 15 are kept for the parts: a field
of an exception class cannot take one with `number:`, nor can `reserved`,
and a field cannot take a part's name. A field without `number:` takes one
from its name, 5535 or more, so it never meets a part. A part whose value
the envelope cannot carry, a NameError's receiver when it is a class, or
that the reading side cannot read back, the top-level `main` of another
interpreter, is left out; its reader then raises "no receiver is
available", as Ruby's does for an exception made without one.

Ruby's exceptions are messages of package `cameo` (`cameo.NoMethodError`),
or of their module's package for those inside one (`json.ParserError`), the
rule a script's classes follow. They declare no fields a script can read:
their readers (`key`, `receiver`, `status`) read the parts.

**`initialize` runs on `new` and on `raise`.** `Exception.new` makes the
exception with its class's name as the message and its fields at their
defaults, then runs `initialize`: the class's own, or Exception's,
`initialize(msg = nil)`, which sets the message. NameError, KeyError and
SystemExit have an `initialize` of their own for what they carry. `super`
from a script's `initialize` reaches these, so `super(msg)` sets the message
and an `initialize` that never calls `super` leaves the class's name, as in
Ruby. `raise E` is `E.new`, `raise E, "x"` is `E.new("x")`. An exception is
open to `initialize` only while `new` builds it; calling `initialize` on an
exception anywhere else raises a CameoError, since an exception never
changes once made.

**A class without an `initialize` of its own takes its fields as
keywords**, as any class does, and its message as Exception's
`initialize` does, the one positional argument: `E.new("broke", status:
500)`, and `E.new(status: 500)` has the class's name as its message. A
keyword that names no field is an ArgumentError when the class declares
fields; one that NameError or KeyError takes (`receiver:`, `key:`) goes to
their `initialize`. As for any class, one positional Hash from a Go host
stands for the keywords.

**`with` makes the copy with changes, the message among them**:
`e.with(status: 502, message: "bad gateway")`. The copy keeps the line and
the cause.

**The cause is Ruby's.** An exception raised in a rescue clause, or in a
method the clause calls, gets the rescued exception as its cause when it
leaves the clause without one. `raise ..., cause: e` gives one, `cause:
nil` none, and `raise e, cause: e` is an ArgumentError; a rescued exception
whose causes lead back to the one raised is not made its cause. A
bare `raise` in a rescue clause raises the rescued exception again; outside
one it raises the RuntimeError "unhandled exception", as before.
Exceptions are immutable, so the exception that gets a cause is a copy:
a local that held it keeps it without one.

**Crossing.** A worker's exception goes to its awaiter in the envelope and
is read back in the awaiting interpreter, with its class found by name and
digest, as an instance's is; `fail promise, E, ...` settles the same way. An
exception of a class no constant names, one `Class.new` made and nothing
bound, cannot be found that way and crosses as it is. An exception is a
value the envelope carries, so it can be an argument to `async`, a result,
or what `resolve` settles with.

**The Go host** gets an exception as the message its class is:
`RunError.Exception()` returns it for the exception that ended a run or a
`Call`, and `ToGo` (and so `Call`'s result) for an exception a script
returns. The host reads the parts and the fields by name with
`protoreflect`. A message of a script's exception class, or of one of
Ruby's, goes back in as the exception. `cameo proto` writes a script's
exception class with its parts and fields, importing `cameo/value.proto`
for the cause.

## Performance

Measured on a 10-core laptop shared with other work, the minimum of 9
interleaved runs of 5 million iterations: `raise MyError` and its rescue
went from 0.64 s to 0.69 s of CPU, `raise ArgumentError, "bad"` from 0.94 s
to 1.02 s, about 8%. Most of what a raise of a class cost more came from
looking `initialize` up to see whether `new` must make the exception;
`raise` reads the classes' own method tables up to Exception instead.

## Shape of the code

- `src/object/value.go`: `Exception` with `Cause`, `Fields`, `Building`,
  `Copy` and `Field`.
- `src/protogen/exception.go`: the parts, the descriptor of an exception
  class, `ExceptionMessage`, `MarshalException`, and `Decoder.Exception`.
  `descriptor.go`, `text.go` and `envelope.go` take exception classes where
  they took classes with fields.
- `src/stdlib/exception.go`: Exception's methods and the `initialize` of
  Exception, NameError, KeyError and SystemExit.
- `src/eval/exceptions.go`: `Exception.new`, the `raise` that knows the
  rescued exception, and `caused`, which gives an exception leaving a
  rescue clause its cause. `compile.go` keeps the rescued exception in
  `Interp.rescuing` while a clause runs; `call.go` and `classes.go` let an
  exception's `initialize` set its fields; `fields.go` reads them and makes
  `with`'s copy; `async.go` sends a worker's exception as its message.
- `src/cameo/cameo.go`: `RunError.Exception`, and exceptions in `ToGo` and
  `ToValue`.
