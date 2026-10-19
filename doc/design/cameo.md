# cameo — the language

cameo is a small language with Ruby's syntax, run by an interpreter written in
Go. Every value is immutable, every object is a protobuf message, and a script
can hand work to other goroutines with `async` and collect it with `await`.
This document is the design: what the language is, what it leaves out, how the
interpreter is built, and in which order.

The front end is `github.com/go-ruby-parser/parser`, a Ruby parser in pure Go
(no cgo, BSD-3-Clause, tested against MRI). cameo adds no syntax of its own:
everything below parses as Ruby, and cameo gives some of it a different
meaning. The constructs cameo introduces — `with`, `async`, `await`,
the typed field helpers — are ordinary method calls to the parser and
intrinsics to the interpreter.

## Source

A script is a `.cameo` file. `cameo script.cameo` runs it. Only current Ruby
syntax is accepted: keyword arguments are passed with `**kwargs` or `key:
value`, never as a trailing positional hash; `=>` is accepted in hash
literals. Legacy spellings the parser still accepts are rejected by cameo's
checker, with the line and the construct named.

What cameo keeps from Ruby:

- literals: `nil`, `true`, `false`, integers, floats, strings with
  interpolation, symbols, arrays, hashes, ranges, `%w[]` and `%i[]`
- local variables and constants
- operators, `if`/`unless`/`elsif`/`else`, `case`/`when`, `case`/`in`
  (pattern matching over immutable data is a natural fit; it lands late, see
  *Milestones*), `while`/`until`, modifier forms, `&&`/`||`/`!`, safe
  navigation `&.`
- `def` with positional, optional, `*rest`, keyword, `**kwrest` and `&block`
  parameters; `return`, `yield`, `block_given?`
- blocks (`{ }` and `do … end`), `->` lambdas, `proc`, `.call`, `&:sym`
- `class` with single inheritance, method overriding and `super`; `module`
  as a namespace and as a mixin through `include` and `extend`
- `begin`/`rescue`/`else`/`ensure`, `raise`, `retry`, exception classes
- sugar the checker rewrites: argument forwarding (`...`, and `*`, `**`,
  `&` without a name), `for x in xs` (as `each`), `alias new old` for a
  name not yet taken, `class A::B` inside an existing `A`, and a top-level
  `def self.x`, which defines a top-level method
- `puts`, `print`, `p`, `warn`

What cameo leaves out, and the checker rejects by name:

- any setter: `def x=(v)`, `attr_writer`, `attr_accessor`
- instance variables, so `@` is free for later use: `initialize` sets a
  field with `self.x = v`, and a field is read with its reader; class
  variables; global variables
- reopening a class: a second `class X` never changes the first, see
  *Classes*; `define_method`,
  `instance_variable_set`, `eval`, `undef`,
  `extend`, singleton classes, `def self.x` on an instance
- mutating methods (`<<`, `push`, `pop`, `delete`, `merge!`, `map!`, every
  bang method) on anything but a local or a field in `initialize`, which
  they rebind, see *Variables rebind*; `freeze` is accepted and changes
  nothing, since everything already is frozen
- threads, fibers, `fork`, files, sockets, `require` of another file
  (`require_relative`, `load`, a library cameo does not build in), `$stdin`. Standard
  output and standard error are the whole of I/O.

## Values

Every value is immutable. The kinds of value are:

| cameo | protobuf scalar or message |
|---|---|
| `nil` | absence (proto3 `optional` not set) |
| `true`, `false` | `bool` |
| Integer | `int64`; arbitrary precision in cameo, so beyond 64 bits it travels in `cameo.Value` as `big_value`, its decimal digits, and an `int` field, a protobuf `int64`, refuses it |
| Float | `double` |
| String | `string` for UTF-8, `bytes` for a binary string; a `string` field takes a string of any encoding whose bytes are UTF-8, a `bytes` field any string |
| Symbol | an `enum` field's value where the class declares one; a `string` everywhere else, see *Symbols* |
| Array | `repeated` |
| Hash | `map<,>`; keys are String, Symbol (stored as `string`), Integer or `bool`, see *Symbols* |
| Set | `repeated`, deduplicated on construction |
| Range | a `Range` message with `from`, `to`, `exclusive` |
| Regexp | in `cameo.Value`, a `Regexp` message: source and options; a MatchData does not travel |
| Time | in `cameo.Value`, a `Time` message: seconds, nanoseconds, UTC offset and zone; there is no field type for it yet |
| an instance of a user class | the message that class declares |
| Class, Proc, Enumerator, Encoding, Pathname | not serialisable; see *Async* |

Equality is structural everywhere: two lists with equal elements are `==`,
two instances of one class with equal fields are `==` and hash alike. There
is no object identity to speak of, so `equal?` does not exist.

### Symbols

Symbols are the language's names: method names in `async worker, :run`,
hash keys, enum values. They are not a protobuf kind. A class stores a
symbol only in an `enum` field, which declares the symbols it accepts (see
*Classes*). Anywhere else a symbol meets protobuf — a Hash key, an Array
element, an `async` argument — it is stored as a `string` and reads back as
a String; a Hash with symbol keys is a `map<string, …>` and comes back with
string keys.

A Hash that is stored in a field or passed to `async` has keys of one type,
and that type is String, Symbol, Integer or `true`/`false`, the keys
protobuf's `map<,>` allows. Any other key raises `TypeError` at the field
assignment or the `async` call. Inside a script a Hash accepts any immutable
key.

The standard library is written in Go and returns new values from everything
that would mutate in Ruby: `list + [x]`, `list.append(x)`, `map.merge(other)`,
`map.put(key, value)`, `map.without(key)`, `set | other`, `string + other`.
`Enumerable` is one Go implementation over Array, Hash, Set and Range: `each`,
`map`, `select`, `reject`, `reduce`, `inject`, `sum`, `min`, `max`, `sort`,
`sort_by`, `group_by`, `each_with_index`, `each_with_object`, `zip`, `take`,
`drop`, `first`, `last`, `find`, `any?`, `all?`, `none?`, `count`, `include?`,
`flat_map`, `to_a`, `to_h`, `to_set`, `uniq`, `partition`, `min_by`,
`max_by`, `each_slice`, `each_cons`, `tally`, `chunk_while`. A class that
includes Enumerable and defines `each` gets the same methods, over what its
`each` yields: one value as itself, several as a list of them, as in Ruby.
`to_enum` and `enum_for` make an enumerator over any method that yields.
`each_with_object` cannot rely on the memo
changing, so the block's value becomes the next memo, as in `reduce`. A
collection that Ruby would change in place returns a copy: `list.append(x)`,
`list.prepend(x)`, `list.without(x)`, `list.without_at(i)`, `map.put(k, v)`,
`map.without(k)`, `set.add(x)`, `set.without(x)`.

## Variables rebind, objects do not change

`a = 1; a = 2` is allowed: assignment binds a name to a value, and binding it
again is the only mutation in the language. Blocks close over their lexical
scope exactly as in Ruby, so a block that assigns an outer variable rebinds it
for the enclosing scope. Nothing else in the program can observe the old
value, since no value refers to a variable.

A call that changes its receiver in Ruby -- `x << y`, `x.push(y)`,
`x.pop`, `h.delete(k)`, `s.add(y)`, any bang method -- is a rebinding too,
when its receiver is a local variable or a field inside `initialize`: x
becomes the changed copy, as `h[k] = v` makes h one. The call's value is
Ruby's: the receiver for most, the removed element for `pop`, `shift` and
`delete`, nil from a bang method that changed nothing. Whether a call
changes its receiver depends on the receiver's kind, which the evaluator
checks: `String#delete` returns a new string, `Integer#<<` shifts, and an
instance's methods are its own. Another variable that held the old value
keeps it, where Ruby would show the change. On any other receiver -- an
element, a constant, a call's result -- such a call is rejected, since
there is no variable to rebind. cameo's own `append`, `prepend`, `put` and
`without` return a copy and change nothing.

### `with`

`value.with(field: expr, …)` returns a copy of `value` with the named fields
replaced. A block computes changes from the current value:
`value.with { |v| { field: expr, … } }` receives the copy so far and returns a
hash of the changes, applied after any keywords. On an instance of a class the
keys must name declared fields and the values must have the declared types.
On a Hash the hash is merged in. On an Array the keys are indices, so the
block form is the one to use. On anything else `with` raises, and so does a
block that returns something other than a hash.

`with` never changes `value`; the caller binds the result:

```ruby
pt = Point.new(x: 1, y: 2)
pt = pt.with(x: 11)                   # Point(x: 11, y: 2); the old pt is gone
pt = pt.with { |p| { y: p.y * 5 } }   # Point(x: 11, y: 10)
```

The keyword form is Ruby's own `Data#with`. `tap` is Ruby's too: it yields
the receiver and returns it unchanged. A `tap` whose block ends in a hash
literal is rejected by the checker, since it reads as the copy `with` makes.

### Chained assignment is sugar for `with`

`a.x.y = 3` is not a setter call. The checker rewrites every `recv.f1.f2….fn
= expr` whose leftmost receiver is a local variable into nested copies and a
rebinding of that variable:

```ruby
a.x.y = 3
# is
a = a.with { |a0| { x: a0.x.with { |a1| { y: 3 } } } }
```

The leftmost receiver must be a local variable (or `self` inside
`initialize`, where `self.x.y = 3` sets field `x`), because the result has
to be bound somewhere. `foo().x
= 3` and `CONST.x = 3` are rejected by the checker with that reason. Index
assignment `a[0] = 1` and `h[:k] = v` desugar the same way, with an index or
key.

## Classes

A class declares typed fields, an optional `initialize`, and methods. It
cannot be reopened with a different body, has no setters, and its instances
are protobuf messages.

```ruby
class Point
  int :x, :y
  string :label, default: ""

  def initialize(x, y, label: "")
    self.x = x
    self.y = y
    self.label = label
  end

  def +(other)
    Point.new(x + other.x, y + other.y)
  end

  def to_s = "(#{x}, #{y})"
end
```

**Fields** are declared with typed helpers, which are class-body intrinsics.
The scalar ones are proto3's scalar types under their own names, with
three of cameo's beside them:

| field type | holds | protobuf |
|---|---|---|
| `string`, `bytes`, `bool` | a String (UTF-8 for `string`), true or false | the same |
| `int` | an Integer of 64 bits, Ruby's Integer as far as protobuf goes | `int64` |
| `int32`, `sint32`, `sfixed32` | an Integer from -2³¹ to 2³¹-1 | the same |
| `int64`, `sint64`, `sfixed64` | an Integer from -2⁶³ to 2⁶³-1 | the same |
| `uint32`, `fixed32` | an Integer from 0 to 2³²-1 | the same |
| `uint64`, `fixed64` | an Integer from 0 to 2⁶⁴-1 | the same |
| `bignum` | an Integer of any size | `string`, its decimal digits |
| `float`, `double` | a Float, Ruby's Float | `double` |
| `float32` | a Float rounded to 32 bits | `float` |

`float` keeps Ruby's meaning, so proto3's 32-bit `float` is spelled
`float32`. An Integer outside its type's range, or a Float beyond
`float32`'s, raises a `RangeError`; an Integer given to a float field
becomes a Float, and a `float32` field keeps the rounded value, so it reads
the same after a round trip. The composite helpers are `list :name, of:
:int32`, `map :name, key: :string, value: :float`, `set :name, of:
:string`, `ref :name, Point` for a nested message, and `enum :name, values:
[:a, :b]` for a field that holds one of the listed symbols. An element type
is any scalar type or a class; a map key is a `string`, a `bool` or an
integer type of 32 or 64 bits, as protobuf requires. Each helper takes one
or more names and an optional `default:`; a helper that declares one field
also takes `number:`, its field number, from 1 to 4096 (see *Field numbers*
under *Protobuf*). A field without a default is
`nil` until `initialize` sets it. Every field gets a reader of the same
name.

`reserved <number> [, :name]` is the one other declaration a class body
takes. It claims a field number, and optionally a field name, that no field
may use, so a field that was removed leaves its number and name behind as
protobuf requires:

```ruby
class Person
  string :name, number: 1    # = 1
  reserved 2, :nickname      # was `string :nickname, number: 2`
  int :age                   # = 15907, derived from "age"
end
```

A `reserved` whose number a field already holds, or whose name a field
declares, is an error at load time, and so is a `number:` that a field
already holds or a `reserved` claims.

**`initialize`** is the one place a field may be set: `self.x = v` there
sets field `x`, and `self.x += 1`, `self.items << v` and `self.home.city =
v` work as they do on a local. A name that is not a field, or a value of the
wrong type, raises. While `initialize` runs, `self` is the instance being
built, so its readers return what has been set so far. cameo has no
instance variables: `@x` is rejected everywhere, which leaves `@` free for
later use. `Class.new(…)`
takes `initialize`'s parameters; a class without `initialize` takes its
fields as keyword arguments. When `initialize` returns, the message is sealed
and becomes the instance.

**Methods** read fields and return values. `self` is the instance. A method
body cannot set a field; the checker rejects `self.x = v` outside
`initialize` at load time, and `with` makes the changed copy instead.

**A class is a value, and `class X … end` binds the name `X` to it**, the
way `x = …` binds a variable. A class is identified by its normalised body:
evaluating the same `class X … end` a second time produces the same class, so
the same file can be loaded twice and every async worker can load the same
class source. Evaluating `class X … end` with a different body produces a
different class and rebinds `X` to it: the first class is not changed, and its
instances keep it as their class and keep its methods. Nothing can reopen a
class, or replace or add a method once its body has run. Within one class
body, or at the top level, a second `def` of a name replaces the first, as
in Ruby; the same `def` evaluated again, as a loop around it does, changes
nothing. There is no `remove_method` or `undef`.

**Inheritance** is single, by `class Child < Parent`. A child inherits fields
and methods, may add fields, and may define a method of a name its parent
defines: a method is defined once *per class*, dispatch is Ruby's, and
`super` calls the parent's. Since a class is a value, `Parent` names
whichever class was bound when `Child` was evaluated; rebinding `Parent`
later does not reach `Child`.

### Modules

A module is a namespace and a mixin, as in Ruby, with the same rule as a
class: it is a value bound to its name, and `include` and `extend` take the
value bound at that moment. `module Geo; class Point; end; end` defines
`Geo::Point`, whose protobuf message is `Point` in package `geo` (nested
modules join with dots). A module may define methods and constants, but no
fields: `string :name` in a module body is an error, since a module is not a
message. `include M` adds `M`'s methods to a class with Ruby's method
resolution order, so `super` in a mixin method reaches the next ancestor;
`extend M` adds them to the class object. `is_a?` and `===` see included
modules. `Module#included` and the other hooks do not exist.

## Blocks, lambdas and control flow

Blocks and lambdas are Ruby's: a block is a closure, `yield` calls it,
`&block` captures it as a Proc, `proc.call` and `proc.()` invoke it. `return`
inside a block returns from the enclosing method; `break` and `next` behave as
in Ruby. Procs are values but not serialisable, so they cannot cross `async`.

Exceptions are Ruby's: `raise "msg"`, `raise ErrorClass, "msg"`,
`rescue Class => e`, `ensure`, `retry`. `StandardError`, `ArgumentError`,
`TypeError`, `KeyError`, `IndexError`, `NoMethodError`, `ZeroDivisionError`,
`RuntimeError`, `StopIteration` and `CameoError` (cameo's own, for checker
and type violations) are predefined; a script can subclass `StandardError`.

## Async

Concurrency is the Cameo module's, which a script includes, at the top level
for every method or in a class body for that class; `Cameo.async` and
`Cameo.await` need no include. A promise is a result to come, and the script
holds it by its id, an Integer:

```ruby
include Cameo
job = async worker, :process, item, retries: 3   # starts worker.process(item, retries: 3)
result = await job                                # the return value, or re-raises
result = await worker, :process, item            # async + await in one step
results = await [job1, job2]                      # each result, in order
results = await :all                             # every call started in this scope
p = promise                                      # a pending promise
resolve p, value                                 # settles it with value
fail p, ArgumentError, "no"                      # settles it with an exception
```

`async` runs one method call on one receiver concurrently, with its own
interpreter state, over copies of the arguments, and returns the call's
promise. `async` and `await` are intrinsics, not methods, so `async xxx,
:yyy, *args` needs no parentheses. The receiver, the method name and the
arguments are serialised through the protobuf value envelope at the `async`
call, and deserialised on the other side; a Proc, a Class or a
block among them is an error at the `async` call site, not inside the job.
The receiver is a value (any serialisable value, including an instance of a
class), and the method is looked up on its class in the worker. Classes are
shared by every worker: the class table is built when the script loads and is
immutable afterwards, so it needs no copying and no lock.

A promise is an Integer so that it travels: a script passes one to a worker,
which settles it with `resolve` or `fail`, and a Go host returns one from a
method and settles it from a goroutine of its own when its I/O is done
(`prog.Promise()`, `Resolve`, `Fail`). That is how a script waits for what
cameo does not do itself: the host's method starts the work and returns at
once, so a script that starts several before it awaits any has them run at
the same time. Every promise of a run, the workers' included, is in one
table; a promise is settled once, and settling it again raises.

`await promise` blocks until the promise is settled and returns its value;
if it failed, `await` raises the same exception class with the same message
in the awaiting scope. Awaiting a promise twice returns the same result,
as long as the method that started it has not returned.

A promise `async` started lives as long as that method: when it returns,
the table forgets the promises it awaited, and those it did not await are
forgotten at their first `await`, wherever the id went. Awaiting a
forgotten promise raises `ArgumentError` ("no promise 7"). The table
therefore holds the promises still in use, not every one a run started,
which a recursion that fans out at every level would fill with millions.
`await [a, b]` waits for each and returns their results in order, raising
the first failure once all are settled. A promise nobody settles blocks its
`await` until the run is stopped.

`await :all` waits for every promise `async` started in the current method
(its blocks included, since a block is part of the method that runs it) or at
the top level, that has not been awaited yet, and returns their results as
a list in start order. If any raised, `await :all` waits for all of them first and
then raises the first exception in start order. A scope that returns with
calls still running leaves them running: a promise is a value, so a
caller that receives one may still await it, and one nobody awaits is
waited for when the run ends, with its failure written to standard error.
`cameo` exits when the script's top-level scope finishes and every call
`async` started has. A call's result has to be able to travel back, so a
method that returns a Proc or a class to a worker raises a `TypeError` there;
`resolve` takes the same values.

A worker has no handle to its parent and no shared memory; what it sends
back is its return value and the promises it settles. Starting calls from
inside a worker is allowed and nests the same way.

A call `async` started waits in the run's queue until one of up to
GOMAXPROCS runner goroutines takes it, the oldest of its shard of the
queue first, or until an
`await` of its promise finds it still queued and runs it on its own
goroutine. A goroutine per call would park, with its stack, every call
whose awaiter runs it, and a recursion that fans out at every level parks
millions. Two rules keep every call able to run: a goroutine about to block
in `await` while calls are queued starts another runner, so runners that
wait on promises only queued calls settle still leave one free; and a
watchdog starts another runner when queued calls have waited 10 ms with no
runner taking one, so calls that never end do not hold the queue up for
good.

## Protobuf

Every class is a protobuf message, which is what makes a cameo value cheap
to hand to Go, to another goroutine, or over a wire.

**Representation.** When a class is loaded the interpreter builds a
`FileDescriptorProto` for it from its field declarations (one message, one
field per declared field, numbered as *Field numbers* says, `optional` for
presence, nested `ref` fields as message fields, each `enum` field as a
nested enum named after the field, with a `<NAME>_UNSPECIFIED = 0` value
that stands for `nil` and the declared symbols upper-cased after it). An
instance holds its fields as cameo values, and builds the
`dynamicpb.Message` over that descriptor on first use and keeps it, which
is sound because the fields never change: the evaluator reads fields
without going through protobuf, and the host gets a `proto.Message` that
is the instance. Lists, maps and sets of scalars map
to `repeated` and `map<,>`. So an instance *is* a `proto.Message`, and
`proto.Marshal` of it is the wire form of that class.

**Field numbers** come from the field's name, unless its declaration gives
one with `number:`. A field declared without `number:` is numbered

    5535 + 60000 * fnv32a(name) / (2³² - 1)

(`protogen.FieldNumber`): a number from 5535 to 65535, picked by the FNV-1a
hash of its name. A number in 19000 to 19999, which protobuf reserves for
itself, moves up by 1000, and a number that a field of the class already
holds, or that `reserved` claims, moves to the next free one, wrapping from
65535 to 5535. `number:` takes 1 to 4096, so a declared number never takes
one derived from a name. A declared number is cheaper on the wire -- one
byte of tag up to 15, two up to 2047, three above, where a derived one
takes three -- and is the choice for a message that is sent often, or one
whose numbers another program fixed.

So adding, removing or reordering fields does not renumber the others, and
two programs that each add a field of a different name to the same class
give it different numbers, but for a hash collision, which a host notices
when it merges the definitions (below). Renaming a field is a schema change,
as in protobuf: the field gets the number of its new name. A removed field
leaves `reserved <number>, :name` behind, its number read off the class's
`.proto`.

**Merging two definitions.** Programs that keep messages in one store may
each have a definition of a message of their own: an older one lacks a
field a newer one added, and two may each add a field the other lacks.
`protogen.Merge(registered, next)` returns the file of a definition that
reads what both write. It matches fields by name, as protobuf JSON does:

- A field both have keeps the registered definition's number and type. The
  other definition must give it the same type -- the same scalar kind, the
  same enum with the same values, or the same message, which may lack
  fields -- the same cardinality and the same oneof. Anything else fails.
- A field only the registered definition has stays.
- A field only the other definition has is added, numbered by
  `protogen.FieldNumber` from its name, whatever number the other
  definition gave it, with the enums and messages it uses and the imports
  they need. It may not be in a oneof, nor have a name the registered
  definition reserves.

When the other definition only lacks fields, the result is the registered
file. The merged message reads the protobuf JSON of both definitions, and
the wire form of both where their numbers agree, which numbers derived from
names always do but for a collision. A host that stores schemas registers
the merged file as its next version: ark's datastore does this for its
document types, keeping the registered `.proto` source and appending the
added fields to it.

**Generating a schema.** `Point.proto` returns the `.proto` text of the
class (for a class inside a module, starting with its `package` line), and `cameo proto script.cameo` writes one `.proto` file holding every
class the script defines, in package `cameo`. A host that compiles that file
with `protoc` gets generated Go types whose wire form matches what the
interpreter produces, so a message can be marshalled on one side and
unmarshalled on the other without any cameo code in between.

**The value envelope.** Values that are not instances — scalars, lists, maps,
sets, ranges — travel in `cameo.Value`, a message with a `oneof` over the
scalar kinds plus `List`, `Map`, `Set`, `Range`, `Time`, `Regexp` and `Object` (a
class name and the packed message). A symbol travels as a string, see *Symbols*.
A hash's default value travels with it; a hash with a default block cannot
travel. It is what `async` serialises, what the
Go host API takes and returns, and what `cameo proto` emits alongside the
classes.

**The Go host API**, package `cameo`:

```go
prog, err := cameo.Load(file, src, stdout, stderr) // parse and check
err        = prog.Run()                            // run once; the classes stay loaded
err        = prog.RunContext(ctx)                  // the same, stopped when ctx ends
value, err := prog.LastValue()                     // the last expression, as p shows it
out, err  := prog.Call(recv, "method", args...)    // recv: a Go value, a class name, a proto.Message
text, err := prog.Proto("Geo::Point")              // the .proto text of a class
files     := prog.ProtoFiles()                     // every package's .proto, and cameo/value.proto
data, err := prog.Encode(v); v, err = prog.Decode(data)   // the envelope
err        = cameo.Run(file, src, stdout, stderr)  // what the binary does
```

Go values convert to cameo values by the table under *Values*: nil, bool,
the integer and float kinds and `*big.Int`, string, `[]byte`, `[]any`, `map[string]any`,
and a `proto.Message` whose full name is a loaded class's message. Results
come back as nil, bool, int64 (a `*big.Int` beyond 64 bits), float64, string, `[]byte`, `[]any`,
`map[any]any`, and a `proto.Message` for an instance; a symbol, range or
class stays an `object.Value`. A Hash's Symbol keys become strings, as
symbols do wherever a value leaves cameo; a Hash whose keys have no Go map
form (a key not comparable in Go, or `:a` beside `"a"`) is an error.

**Host modules.** A host gives the script what cameo does not have, a
service or a tool, through a module of its own, defined after `Load` and
before `Run`:

```go
mod := prog.Module("Spicy")                       // the constant Spicy, at the top level
toolError := mod.Exception("ToolError")           // Spicy::ToolError, a StandardError
mod.Def("tools", func(args []any, kwargs map[string]any) (any, error) {
	return nil, toolError.New("no tool %v", args[0]) // raises Spicy::ToolError
})
```

The script calls `Spicy.tools("search", query: "x")`. A host method gets
its arguments as results come back, but with a symbol as a string, as
everywhere a value leaves cameo, and a hash with string or symbol keys as
a `map[string]any`; what it returns converts as `Call`'s arguments do. An
error it returns raises an exception, of the class an `Exception` names or
`RuntimeError`. A host module is a value like a builtin one: no script
redefines or reopens it. Workers that `async` started reach it too, each on
its own goroutine, so a host method may run on several at once.

**Native classes.** A host gives the script a class of its own whose data
layout is a protobuf message. The message is written in a `.proto` file
and generated with `protoc-gen-go`. The host registers the generated type
under a name and adds methods in Go:

```proto
// geo.proto
syntax = "proto3";
package example.geo;
option go_package = "example.com/geo/geopb";

message Point { int64 x = 1; int64 y = 2; }
```

```go
func init() {
	cameo.Register("Geo::Point", &geopb.Point{}).
		DefineMethod("norm2", func(p *geopb.Point, args []any, kwargs map[string]any) (any, error) {
			return p.X*p.X + p.Y*p.Y, nil
		}).
		DefineMethod("moved", func(p *geopb.Point, args []any, kwargs map[string]any) (any, error) {
			p.X += args[0].(int64) // p is a copy; the changed copy is the result
			return p, nil
		}).
		DefineClassMethod("origin", func(args []any, kwargs map[string]any) (any, error) {
			return &geopb.Point{}, nil
		})
}
```

```ruby
pt = Geo::Point.new(x: 3, y: 4)   # the fields as keywords, as for any class without initialize
pt.norm2                          # 25
pt.moved(1).x                     # 4; pt.x is still 3
Geo::Point.origin.with(y: 2)      # with, ==, inspect, to_proto: what every instance has
```

Registration rules:
- **The class's fields are the message's**: every proto3 scalar type,
  repeated fields as lists, maps with a scalar key, enums numbered from 0
  without a gap (their values become symbols, lowercase, without the prefix
  the names share: `COLOR_RED` is `:red`), and fields of messages whose
  class was registered before, or of the message itself. A `oneof` is not
  supported.
- **Unset fields read as Go does.** A field without presence that is not
  set reads as its zero value, so `Point.new.x` is `0`. A field with
  presence (`optional`, a message) reads as `nil`.
- **Methods get copies.** A method gets `self` as the generated type, a copy
  it may change without changing the instance. The arguments, keywords,
  result and errors convert as a host method's do. A returned message of a
  registered type becomes an instance of its class, so a "changed" instance
  is a returned copy.
- **Instances leave cameo as the generated type.** `ToGo` and `Call`'s
  results give a `*geopb.Point`, and a `*geopb.Point` passed in becomes an
  instance. `async` carries instances to workers like any other.
- **Names make modules.** A qualified name makes the modules it needs, and
  classes registered under one namespace share its module.
- **Registered names are closed.** As builtin ones are, a registered class
  or module is a constant of every program in the process. No script
  redefines it, nor defines anything inside it: `class Geo::Point … end`
  and `module Math::Extra … end` are errors. `Program.Module("Geo")` gives
  a program a module with methods of its own that still holds the
  registered classes.
- **Register before the first Load**, from an `init` function. `Register`
  panics after that, and when a name or a message does not fit: both are
  mistakes in the host's code, not in a script.

**Two consequences of the wire form.** A protobuf map has no order and
cannot tell an absent repeated field from an empty one. So a map's identity
in cameo does not depend on insertion order, a decoded map comes back
sorted by key, and a list, set or map field never holds nil: given nil it
holds the empty collection. A value therefore survives `from_proto(to_proto)`
equal to itself.

## The Ruby adapter

A script that uses only cameo's subset is valid Ruby, so it can run under MRI
with a library that supplies cameo's intrinsics. That is `parts/ruby/cameo`,
a gem with an `Cameo` module, and it is explicitly second in line behind the
Go interpreter. Its shape:

- `Cameo::Object` as the base class: the typed field helpers (`string :name`
  and the rest) define readers and a frozen initializer, `with` builds a copy, `==` and `hash` are structural, and
  `to_proto` returns the message through `google-protobuf`, built from the
  same descriptor the Go side generates. Values are deep-frozen on
  construction through `simple-immutable`, an existing ark part.
- `Cameo::Kernel` with `async`, `await` and `await :all` on threads, with the
  same serialisation rule enforced by marshalling the arguments through the
  value envelope.
- `cameo-rb script.cameo` runs a script with the module loaded.

What the adapter cannot provide is the `a.x.y = 3` rewrite, since a library
cannot rebind a caller's local. Scripts that must run under both write the
`with` form; the adapter may later gain a source rewriter.

## Architecture

One Go module, `github.com/cameo-lang/cameo`, with the packages under `src/`:

| Package | Does |
|---|---|
| `cameo` | the host API: `Load`, `Run`, `Program`, `Value` |
| `rx` | translates a Ruby regular expression into one for Go's regexp, and names what it uses that Go's cannot do |
| `syntax` | wraps the parser; normalises its tree into cameo's own AST, rewrites `a.x.y =` and index assignment into copies, rejects what cameo leaves out, with line numbers; hashes class bodies so equal definitions are one class |
| `object` | the value model: a `Value` is a tagged 24-byte struct (kind, integer payload, pointer), so an integer, a float, a boolean and nil never touch the heap and a string shares its bytes; kinds, structural equality, hashing, the class table, dynamic messages, conversion to and from `cameo.Value` and Go values |
| `eval` | the evaluator: compiles each node to a closure on first use (`compile.go`), with a tree walker for the rare kinds; scopes, method dispatch with a per-class lookup cache, blocks, exceptions, intrinsics (`with`, `async`, `await`, the field helpers) |
| `stdlib` | Kernel, Integer, Float, String, Symbol, Array, Hash, Set, Range, Time, Math, Regexp, MatchData, Enumerable, Proc, the exception classes |
| `eval/async.go` | promises and the run's table of them, the per-scope list `await :all` reads, the worker interpreter a call runs in |
| `eval/sched.go` | the queue of calls `async` started and the runner goroutines that take them |
| `protogen` | descriptors from class declarations, `.proto` text, the `cameo.Value` envelope (`proto/cameo.proto`, compiled in) |
| `cmd/cameo` | the binary: `cameo script.cameo`, `cameo check script.cameo`, `cameo proto script.cameo` |

Tests are the example corpus first: `examples/<name>.cameo` beside
`examples/<name>.expected`, run by `go test` which executes each script and
compares standard output; a script that must fail carries `examples/<name>.expected-error`
with the expected error instead. The same corpus is the Ruby adapter's suite
later, so the two implementations are held to one behaviour. Package tests
cover what a script cannot reach: descriptor shapes, value conversion, the
checker's rejection messages.

## Milestones

Each is a pull request that leaves `make check` green and the example corpus
passing for what it covers.

0. **Skeleton** (this document): component, manifest, Makefile, the parser
   as a dependency, `cameo check` that parses a script and reports syntax
   errors, the example corpus as the specification, and a test that every
   example parses.
1. **Front end**: `syntax` — cameo's AST, the rejection list, the copy
   rewrite, with the checker's messages tested.
2. **Evaluator core**: literals, variables, operators, control flow, methods,
   blocks, lambdas, exceptions, `puts`/`print`/`p`/`warn`. `cameo script.cameo`
   runs scripts without classes.
3. **Standard library**: String, Integer, Float, Symbol, Array, Hash, Set,
   Range, Enumerable.
4. **Classes**: typed fields as dynamic messages, `initialize`, readers,
   `with`, chained assignment, classes as values bound to their name,
   inheritance with overriding, modules as namespaces and mixins.
5. **Protobuf**: `.proto` generation with enums, packages and `reserved`,
   the value envelope, the Go host API, `cameo proto`.
6. **Async**: the scheduler, `async`, `await`, `await :all`, the
   serialisation rule.
7. **Ruby adapter**: `parts/ruby/cameo`, run against the same corpus.

## Decisions taken

Choices with real trade-offs, decided with the human so they are not
re-opened by accident:

- **Overriding** is per class: a child may define a method its parent
  defines, and dispatch is dynamic. The alternative, one definition per
  hierarchy with static dispatch, was rejected as too far from Ruby.
- **Modules** are in the first release as namespaces and mixins. Namespaces
  alone would have been cheaper; mixins bring method resolution order, which
  the evaluator has to implement anyway for inheritance.
- **Symbols** have no protobuf kind of their own. An `enum` field declares
  the symbols it stores; everywhere else a symbol becomes a string at the
  protobuf boundary.
- **Hash keys** are the four protobuf allows. A second representation for
  arbitrary keys was rejected as not worth its cost.
- **Field numbers** come from a hash of the field's name, or from
  `number:` (1 to 4096). Numbering in declaration order made the number
  depend on the field's position, so reordering fields broke the wire
  form, and two programs that each appended a field to a shared message
  gave two fields one number. `reserved` keeps a removed field's number
  and name out of use.
- **`send`** calls a method by name, as Ruby's does, and so do
  `public_send` and `__send__`, since every method is public. It reaches
  `method_missing` and the top-level methods a bare call reaches. It never
  rebinds its receiver, so it refuses what a call on a receiver cameo
  cannot rebind refuses, `push` and the bang methods among them, and
  `initialize`, which only `new` runs on an instance under construction.
  Leaving it out cost scripts that pick a method from data; the parser
  cannot see the name, so the refusal happens when the call runs.
- **Concurrency is a module a script includes.** `include Cameo` brings
  `async`, `await`, `promise`, `resolve` and `fail`; without it they are
  unknown, so a script states that it runs work concurrently. `async`
  returns a promise's id, an Integer, rather than a process object, so a
  promise travels to a worker or to the host like any other value, and
  whoever has the work settles it: a worker by returning, a script with
  `resolve` and `fail`, a Go host with `Promise.Resolve` and `Fail`. A
  host's I/O goes through promises, since cameo does none itself. `fail`
  with an Integer first settles a promise; with anything else it raises,
  as Ruby's `fail` does.
- **No instance variables.** `initialize` sets a field with `self.x = v`,
  and a field is read with its reader, so `@` is free for later use. Ruby
  code that sets `@x` in `initialize` no longer runs unchanged; it never
  could outside `initialize`.
- **Field types** are proto3's scalar types under their own names, except
  that `float` keeps Ruby's meaning, a 64-bit `double`, and proto3's 32-bit
  float is `float32`; following proto3 to the letter would have changed
  every existing `float` field's precision and wire type. Integers stay of
  any size, as in Ruby, and a `bignum` field holds one as its decimal
  digits, the form the value envelope already uses; dropping big integers
  was rejected, since 24 of the benchmark cases use them and the overflow
  checks would stay to raise instead.
- **Classes are values** bound to their name: a second definition with the
  same body is the same class, one with a different body rebinds the name,
  and nothing reopens a class. `Class.new(Base) { ... }` builds one at run
  time: its block is the class body, it is identified by the block's
  digest, and it takes the name of the first constant it is bound to.
  `Module.new` stays out, since its use is to include or extend at run
  time.
- **Only current Ruby syntax**: keyword arguments are keywords, never a
  trailing hash; `=>` stays for hash literals.
- **Native classes** take their fields from a generated protobuf message
  and are registered for the whole process, as builtin classes are, before
  the first program loads. Classes per program were left out: a class is
  a value scripts and hosts pass around, and one class per name in a
  process keeps `async` and the wire form simple. A registered namespace is
  closed to scripts, and so, now, is a builtin one.
- **The standard library follows CRuby's benchmarks** (`benchmarks/ruby/`):
  - `Array` and `Hash` are the Ruby-visible names of cameo's lists and maps,
    so `[].class`, inspect and error messages match Ruby; the field types
    keep their protobuf names, `list` and `map`.
  - Random numbers come from Go's `math/rand/v2`: one generator per
    interpreter, seeded by `srand`. Reproducing Ruby's Mersenne Twister
    sequence was rejected as not worth its code, so seeded scripts do not
    print Ruby's numbers.
  - `String#succ` follows Ruby's full rules, and ranges over strings iterate
    with it: `include?` walks such a range, `cover?` and `===` compare its
    ends.
  - `Time` is an immutable value with fixed UTC offsets and the local zone.
    `utc` and `localtime`, which change their receiver in Ruby, return a
    new time as `getutc` and `getlocal` do, and `subsec` is a Float.
  - `Hash.new` takes a default value or a default block. The syntax check
    rejects a block that stores into the hash, as it rejects any mutation;
    the block returns the default instead. The default travels with the
    copies `[]=`, `merge` and `compact` make, and a hash built anew has
    none, as in Ruby.
  - `Process.clock_gettime` reads the realtime and the monotonic clock, in
    any of Ruby's units, so a script can time itself; the rest of `Process`
    stays out, since cameo starts no processes.
  - Left out: `Struct` (its members are untyped and its point is setters;
    cameo classes declare typed fields), `Rational` and `Complex` (with
    their literals), and `RUBY_VERSION`.
- **The language follows CRuby's benchmarks too**, a second round over
  what they use beyond the standard library:
  - Taken in: a mutating call on a local (`x << y`, `x.push(y)`,
    `x.map!`, `s.add(y)`) rebinds the local, as `h[k] = v` already does;
    `freeze`, `frozen?` and `compare_by_identity` as no-ops, since values
    are immutable and have no identity; argument forwarding (`...`, `*`,
    `**`, `&`); a top-level `def self.x`; `:sym.to_proc` as a value;
    `Class.new(Base) { ... }` as a class value; regular expressions,
    on Go's regexp; integers of any size; lazy enumerators;
    `require` of a library cameo builds in, with `CGI.escape` and its
    kin; `method_missing`; string encodings to and from bytes; `pack`;
    `for` loops, `alias` and `class A::B`; path arithmetic without I/O
    (`File.join`, `Pathname`).
  - Regular expressions run on Go's regexp, RE2, which matches in linear
    time and has no backtracking. `rx` translates Ruby's syntax: `^` and
    `$` at every line, `m` as Go's `s`, `x`, `\h`, `{,n}`, `\e`, `\u{}`,
    `\xHH` sequences, Ruby's property names; a named group alone captures.
    A pattern that needs backtracking (a backreference, lookahead or
    lookbehind, an atomic group, a possessive quantifier, a subexpression
    call, `\G`, `\K`, `\Z`, class intersection) is refused with "This
    regexp is not supported in Cameo": a literal by the syntax check, one
    built at run time with a NotImplementedError. A Regexp is an immutable
    value of source and options, and every String method that takes a
    pattern takes a string or a regexp. There are no `$~` or `$1` globals:
    a match is what `match` returns, or a block's argument. A search from a
    position past 0 reads the text from there, so `^`, `\A` and `\b` take
    that position for the start. An Onigmo engine (go-ruby-regexp) was
    tried and set aside for now: it needs Go 1.27.
  - An Integer has two forms, `int64` and, once arithmetic leaves 64 bits,
    a `big.Int`; a result that fits moves back, so a number has one form
    and equality and hash keys need no conversion. The 64-bit fast paths
    check for overflow and hand that case to the ordinary call.
  - An Enumerator produces its elements on demand, from a Go function
    that hands them to a consumer one by one; a consumer that has had
    enough returns a stop of its own from it, which only it recognises, so
    `take` ends an endless source without ending a pipeline it is nested
    in. `lazy` gives an `Enumerator::Lazy`, whose `map`, `select`, `take`
    and the rest make another and do no work until `first`, `to_a`,
    `force` or `each` asks; a blockless `cycle` or `then` gives an eager
    Enumerator. There is no external iteration (`next`, `peek`).
  - A user class that includes Enumerable and defines `each` gets
    Enumerable's methods. They run on an enumerator over its `each`, so
    `first`, `find`, `take_while` and `lazy` stop once they have their
    answer and an endless `each` works with them; the rest read all it
    yields. A builtin passes a Go function as the block, which the
    evaluator calls like any other.
  - `require` of a library cameo builds in (`set`, `time`, `cgi`,
    `cgi/escape`, `pp`, `json`, `benchmark`) returns true the first time and false after, and
    any other is a LoadError: a script is one file. The libraries are
    there whether or not a script requires them. `time` brings
    `Time.parse`, `Time.iso8601` and `Time.strptime`, and `Time.new` reads
    a string as Ruby 3.2's does; `cgi` brings `CGI.escapeHTML`,
    `unescapeHTML`, `escape`, `unescape`, the URI-component pair,
    `escapeElement`, `unescapeElement`, `rfc1123_date` and `pretty`.
    Ruby writes the last three with regexps that use lookaround, which
    cameo's regexps lack, so they are scanned by hand in Go. `json`
    follows Ruby's json 2.9.1 to its output and error messages. An
    instance of a cameo class without a `to_json` of its own is written as
    an object of its fields: the fields are typed and named, so that is
    what the value is, while Ruby's fallback, the string of `to_s`, loses
    it. The values are cameo's, not protojson's, which would quote 64-bit
    integers, rename fields and drop defaults, so an instance and a Hash
    of its fields write the same JSON. A class's own `to_json` gets the
    generator's options as a Hash, which Ruby's `to_json` methods take as
    well, in place of a `JSON::State`. `benchmark` follows Ruby's
    benchmark 0.4.0 to its output, in Go: it reads getrusage(2) and the
    monotonic clock itself and collects garbage between bmbm's rehearsal
    and its run, so a script needs neither `Process.times` nor `GC`.
  - A call to a method a class lacks goes to the class's own
    `method_missing`, with the name as a symbol, the arguments and the
    block; `super` from it reaches Object's, which raises NoMethodError.
    `respond_to?` asks `respond_to_missing?` for a name it does not find.
  - A String carries its encoding, as Ruby's does: UTF-8, ASCII-8BIT
    (binary) or US-ASCII, the value's kind. Literals are UTF-8; `.b`,
    `pack` and a protobuf `bytes` field make binary strings; `force_encoding`
    relabels the same bytes and `encode` converts them. A binary or
    US-ASCII string has one character per byte, changes case in ASCII
    alone, prints a byte past 7 bits as `\xNN`, and gives its methods'
    results its encoding; `+`, `<<`, `join` and interpolation take the
    encoding their parts are compatible in, or raise
    Encoding::CompatibilityError. `valid_encoding?` and `scrub` deal with
    bytes an encoding does not allow. `force_encoding` rebinds a local, as
    Ruby's changes its receiver; on anything else it returns the relabelled
    string, which is all Ruby's leaves to see. Integer and Symbol `to_s`
    give UTF-8 where Ruby gives US-ASCII; only `encoding` tells.
  - Paths are arithmetic, not I/O: `File.join`, `basename`, `dirname`,
    `extname`, `split`, `expand_path` and `absolute_path`, and `Pathname`,
    an immutable path whose `+`, `join`, `cleanpath`, `parent` and
    `relative_path_from` follow Ruby's pathname.rb. `expand_path` reads the
    working directory and `$HOME` to resolve a relative path or `~`;
    nothing reads a file. `__FILE__` is the script's path and `__dir__`
    its directory.
  - `with` makes the copy with changes, from keywords (Ruby's
    `Data#with`) or from the hash its block returns; `tap` is Ruby's,
    yielding the receiver and returning it. Until this round `tap` made the
    copy, which left Ruby's `tap` idiom out.
  - A mutating call on a temporary (`s.dup.merge!(x)`, `[1, 2].push(3)`,
    `foo.bar[0] = 1`) stays rejected: there is no variable to rebind, and
    the non-mutating form (`merge`, `+`, `with`) says what is meant.
  - The `cameo` binary sets the collector to GOGC=400 with a soft memory
    limit of 1 GB, unless the environment sets GOGC or GOMEMLIMIT. On
    CRuby's benchmarks that takes a quarter off the total time (5000! in a
    third of it) while the median script's memory is unchanged; the limit
    keeps a large heap from growing fivefold. A program that embeds cameo
    keeps Go's defaults.
  - Kept out: `GC`, `ENV`, metaprogramming (`define_method`, `send`,
    `eval`, `instance_variable_set`), setters and untyped fields, fibers
    and threads, and anything that touches the file system.
  - Maps and sets are versions of an append-only table (`object/table.go`),
    so a copy with one more key costs O(1) and building a map key by key
    is linear, not quadratic.
