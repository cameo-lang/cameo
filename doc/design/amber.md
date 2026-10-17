# amber — the language

amber is a small language with Ruby's syntax, run by an interpreter written in
Go. Every value is immutable, every object is a protobuf message, and a script
can hand work to other goroutines with `async` and collect it with `await`.
This document is the design: what the language is, what it leaves out, how the
interpreter is built, and in which order.

The front end is `github.com/go-ruby-parser/parser`, a Ruby parser in pure Go
(no cgo, BSD-3-Clause, tested against MRI). amber adds no syntax of its own:
everything below parses as Ruby, and amber gives some of it a different
meaning. The constructs amber introduces — `tap` with a hash, `async`, `await`,
the typed field helpers — are ordinary method calls to the parser and
intrinsics to the interpreter.

## Source

A script is a `.arb` file. `amber script.arb` runs it. Only current Ruby
syntax is accepted: keyword arguments are passed with `**kwargs` or `key:
value`, never as a trailing positional hash; `=>` is accepted in hash
literals. Legacy spellings the parser still accepts are rejected by amber's
checker, with the line and the construct named.

What amber keeps from Ruby:

- literals: `nil`, `true`, `false`, integers, floats, strings with
  interpolation, symbols, arrays, hashes, ranges, `%w[]` and `%i[]`
- local variables, constants, instance variables inside `initialize`
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
- `puts`, `print`, `p`, `warn`

What amber leaves out, and the checker rejects by name:

- any setter: `def x=(v)`, `attr_writer`, `attr_accessor`
- instance variables outside `initialize`; class variables; global variables
- reopening a class: a second `class X` never changes the first, see
  *Classes*; a second `def` of one name in a class body; `define_method`,
  `method_missing`, `send`, `instance_variable_set`, `eval`, `alias`, `undef`,
  `extend`, singleton classes, `def self.x` on an instance
- mutating methods: `<<`, `push`, `pop`, `delete`, `merge!`, `map!`, every
  bang method, `freeze` (everything already is)
- threads, fibers, `fork`, files, sockets, `require`, `$stdin`. Standard
  output and standard error are the whole of I/O.

## Values

Every value is immutable. The kinds of value are:

| amber | protobuf scalar or message |
|---|---|
| `nil` | absence (proto3 `optional` not set) |
| `true`, `false` | `bool` |
| Integer | `int64` |
| Float | `double` |
| String | `string` |
| Symbol | an `enum` field's value where the class declares one; a `string` everywhere else, see *Symbols* |
| Array | `repeated` |
| Hash | `map<,>`; keys are String, Symbol (stored as `string`), Integer or `bool`, see *Symbols* |
| Set | `repeated`, deduplicated on construction |
| Range | a `Range` message with `from`, `to`, `exclusive` |
| an instance of a user class | the message that class declares |
| Class, Proc, AsyncProcess | not serialisable; see *Async* |

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
`max_by`, `each_slice`, `each_cons`, `tally`, `chunk_while`. There are no
enumerators: a method that iterates returns a list when called without a
block, so `each_slice(2).map { … }` works and `each_with_index.to_a` is
`each_with_index` with a block. `each_with_object` cannot rely on the memo
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

### `tap`

`value.tap { |v| { field: expr, … } }` returns a copy of `value` with the
named fields replaced. The block receives the current value and returns a hash
of the changes; it may use `v` to compute them. On an instance of a class the
keys must name declared fields and the values must have the declared types.
On a Hash the hash is merged in. On an Array the keys are indices. On anything
else `tap` raises. A block that returns something other than a hash raises.

`tap` never changes `value`; the caller binds the result:

```ruby
pt = Point.new(x: 1, y: 2)
pt = pt.tap { |p| { x: p.x + 10 } }   # Point(x: 11, y: 2); the old pt is gone
```

### Chained assignment is sugar for `tap`

`a.x.y = 3` is not a setter call. The checker rewrites every `recv.f1.f2….fn
= expr` whose leftmost receiver is a local variable into nested `tap` calls
and a rebinding of that variable:

```ruby
a.x.y = 3
# is
a = a.tap { |a0| { x: a0.x.tap { |a1| { y: 3 } } } }
```

The leftmost receiver must be a local variable (or an instance variable
inside `initialize`), because the result has to be bound somewhere. `foo().x
= 3` and `CONST.x = 3` are rejected by the checker with that reason. Index
assignment `a[0] = 1` and `h[:k] = v` desugar the same way, through
`tap` with an index or key.

## Classes

A class declares typed fields, an optional `initialize`, and methods. It
cannot be reopened with a different body, has no setters, and its instances
are protobuf messages.

```ruby
class Point
  int :x, :y
  string :label, default: ""

  def initialize(x, y, label: "")
    @x = x
    @y = y
    @label = label
  end

  def +(other)
    Point.new(x + other.x, y + other.y)
  end

  def to_s = "(#{x}, #{y})"
end
```

**Fields** are declared with typed helpers, which are class-body intrinsics:
`string`, `int`, `float`, `bool`, `bytes`, `list :name, of: :int`,
`map :name, key: :string, value: :float`, `set :name, of: :string`, `ref
:name, Point` for a nested message, and `enum :name, values: [:a, :b]` for
a field that holds one of the listed symbols. Each takes one or more names
and an optional `default:`. A field without a default is `nil` until `initialize`
sets it. Every field gets a reader of the same name. The type names double
as the protobuf types the class generates, see *Protobuf*.

`reserved <number> [, :name]` is the one other declaration a class body
takes. It claims a field number, and optionally a field name, that no field
may use, so a field that was removed leaves its number and name behind as
protobuf requires. A field declared after a `reserved` takes the next number
the reservation does not claim:

```ruby
class Person
  string :name            # = 1
  reserved 2, :nickname   # was `string :nickname`
  int :age                # = 3, as it was before nickname went
end
```

A `reserved` whose number a field already holds, or whose name a field
declares, is an error at load time.

**`initialize`** is the one place instance variables may be assigned, and
`@x = v` there sets field `x`; a name that is not a field, or a value of the
wrong type, raises. Reading `@x` anywhere reads the field. `Class.new(…)`
takes `initialize`'s parameters; a class without `initialize` takes its
fields as keyword arguments. When `initialize` returns, the message is sealed
and becomes the instance.

**Methods** read fields and return values. `self` is the instance. A method
body cannot assign instance variables; the checker rejects it at load time.

**A class is a value, and `class X … end` binds the name `X` to it**, the
way `x = …` binds a variable. A class is identified by its normalised body:
evaluating the same `class X … end` a second time produces the same class, so
the same file can be loaded twice and every async worker can load the same
class source. Evaluating `class X … end` with a different body produces a
different class and rebinds `X` to it: the first class is not changed, and its
instances keep it as their class and keep its methods. Nothing can reopen a
class, replace a method, or add one; within one class body a second `def` of
the same name is an error. There is no `remove_method`, `undef` or `alias`.

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
`RuntimeError`, `StopIteration` and `AmberError` (amber's own, for checker
and type violations) are predefined; a script can subclass `StandardError`.

## Async

`AsyncProcess` is a native class. A process runs one method call on one
receiver on its own goroutine, with its own interpreter state, over copies of
the arguments:

```ruby
job = async worker, :process, item, retries: 3   # starts worker.process(item, retries: 3)
result = await job                                # the return value, or re-raises
result = await worker, :process, item            # async + await in one step
results = await :all                             # every process started in this scope
```

`async` and `await` are intrinsics, not methods, so `async xxx, :yyy, *args`
needs no parentheses. The receiver, the method name and the arguments are
serialised through the protobuf value envelope before the goroutine starts,
and deserialised on the other side; a Proc, a Class, an AsyncProcess or a
block among them is an error at the `async` call site, not inside the job.
The receiver is a value (any serialisable value, including an instance of a
class), and the method is looked up on its class in the worker. Classes are
shared by every worker: the class table is built when the script loads and is
immutable afterwards, so it needs no copying and no lock.

`await job` blocks until the process finishes and returns its value; if the
method raised, `await` raises the same exception class with the same message
in the awaiting scope. Awaiting a process twice returns the same result.

`await :all` waits for every process started in the current method (its
blocks included, since a block is part of the method that runs it) or at
the top level, that has not been awaited yet, and returns their results as
a list in start order. If any raised, `await :all` waits for all of them first and
then raises the first exception in start order. A scope that returns with
processes still running leaves them running: a process is a value, so a
caller that receives one may still await it, and one nobody awaits is
waited for when the run ends, with its failure written to standard error.
`amber` exits when the script's top-level scope finishes and every process
has. A process's result has to be able to travel back, so a method that
returns a Proc or a class to a process raises a `TypeError` there.

A process has no handle to its parent, no shared memory, and no way to send
anything but its return value. Starting processes from inside a process is
allowed and nests the same way.

## Protobuf

Every class is a protobuf message, which is what makes an amber value cheap
to hand to Go, to another goroutine, or over a wire.

**Representation.** When a class is loaded the interpreter builds a
`FileDescriptorProto` for it from its field declarations (one message, one
field per declared field, numbered in declaration order, `optional` for
presence, nested `ref` fields as message fields, each `enum` field as a
nested enum named after the field, with a `<NAME>_UNSPECIFIED = 0` value
that stands for `nil` and the declared symbols upper-cased after it). An
instance holds its fields as amber values, and builds the
`dynamicpb.Message` over that descriptor on first use and keeps it, which
is sound because the fields never change: the evaluator reads fields
without going through protobuf, and the host gets a `proto.Message` that
is the instance. Lists, maps and sets of scalars map
to `repeated` and `map<,>`. So an instance *is* a `proto.Message`, and
`proto.Marshal` of it is the wire form of that class.

**Field numbers follow declaration order**, skipping the numbers `reserved`
claims, so removing a field is a schema-compatible change when its number is
reserved, and appending one is always compatible. Reordering or renaming
fields in the amber source is a schema change, exactly as in protobuf; a
host program that wants to catch that generates the `.proto` once (below),
keeps it, and diffs against it.

**Generating a schema.** `Point.proto` returns the `.proto` text of the
class (for a class inside a module, starting with its `package` line), and `amber proto script.arb` writes one `.proto` file holding every
class the script defines, in package `amber`. A host that compiles that file
with `protoc` gets generated Go types whose wire form matches what the
interpreter produces, so a message can be marshalled on one side and
unmarshalled on the other without any amber code in between.

**The value envelope.** Values that are not instances — scalars, lists, maps,
sets, ranges — travel in `amber.Value`, a message with a `oneof` over the
scalar kinds plus `List`, `Map`, `Set`, `Range` and `Object` (a class name
and the packed message). A symbol travels as a string, see *Symbols*. It is what `async` serialises, what the
Go host API takes and returns, and what `amber proto` emits alongside the
classes.

**The Go host API**, package `amber`:

```go
prog, err := amber.Load(file, src, stdout, stderr) // parse and check
err        = prog.Run()                            // run once; the classes stay loaded
out, err  := prog.Call(recv, "method", args...)    // recv: a Go value, a class name, a proto.Message
text, err := prog.Proto("Geo::Point")              // the .proto text of a class
files     := prog.ProtoFiles()                     // every package's .proto, and amber/value.proto
data, err := prog.Encode(v); v, err = prog.Decode(data)   // the envelope
err        = amber.Run(file, src, stdout, stderr)  // what the binary does
```

Go values convert to amber values by the table under *Values*: nil, bool,
the integer and float kinds, string, `[]byte`, `[]any`, `map[string]any`,
and a `proto.Message` whose full name is a loaded class's message. Results
come back as nil, bool, int64, float64, string, `[]byte`, `[]any`,
`map[any]any`, and a `proto.Message` for an instance; a symbol, range or
class stays an `object.Value`.

**Two consequences of the wire form.** A protobuf map has no order and
cannot tell an absent repeated field from an empty one. So a map's identity
in amber does not depend on insertion order, a decoded map comes back
sorted by key, and a list, set or map field never holds nil: given nil it
holds the empty collection. A value therefore survives `from_proto(to_proto)`
equal to itself.

## The Ruby adapter

A script that uses only amber's subset is valid Ruby, so it can run under MRI
with a library that supplies amber's intrinsics. That is `parts/ruby/amber`,
a gem with an `Amber` module, and it is explicitly second in line behind the
Go interpreter. Its shape:

- `Amber::Object` as the base class: the typed field helpers (`string :name`
  and the rest) define readers and a frozen initializer, `tap` with a block
  returning a hash builds a copy, `==` and `hash` are structural, and
  `to_proto` returns the message through `google-protobuf`, built from the
  same descriptor the Go side generates. Values are deep-frozen on
  construction through `simple-immutable`, an existing ark part.
- `Amber::Kernel` with `async`, `await` and `await :all` on threads, with the
  same serialisation rule enforced by marshalling the arguments through the
  value envelope.
- `amber-rb script.arb` runs a script with the module loaded.

What the adapter cannot provide is the `a.x.y = 3` rewrite, since a library
cannot rebind a caller's local. Scripts that must run under both write the
`tap` form; the adapter may later gain a source rewriter.

## Architecture

One Go module, `github.com/radiospiel/amber`, with the packages under `src/`:

| Package | Does |
|---|---|
| `amber` | the host API: `Load`, `Run`, `Program`, `Value` |
| `syntax` | wraps the parser; normalises its tree into amber's own AST, rewrites `a.x.y =` and index assignment into `tap`, rejects what amber leaves out, with line numbers; hashes class bodies so equal definitions are one class |
| `object` | the value model: a `Value` is a tagged 24-byte struct (kind, integer payload, pointer), so an integer, a float, a boolean and nil never touch the heap and a string shares its bytes; kinds, structural equality, hashing, the class table, dynamic messages, conversion to and from `amber.Value` and Go values |
| `eval` | the evaluator: compiles each node to a closure on first use (`compile.go`), with a tree walker for the rare kinds; scopes, method dispatch with a per-class lookup cache, blocks, exceptions, intrinsics (`tap`, `async`, `await`, the field helpers) |
| `stdlib` | Kernel, Integer, Float, String, Symbol, Array, Hash, Set, Range, Enumerable, Proc, the exception classes |
| `eval/async.go` | the scheduler: `AsyncProcess`, the per-scope process list, the worker interpreter a process runs in, goroutine lifecycle |
| `protogen` | descriptors from class declarations, `.proto` text, the `amber.Value` envelope (`proto/amber.proto`, compiled in) |
| `cmd/amber` | the binary: `amber script.arb`, `amber check script.arb`, `amber proto script.arb` |

Tests are the example corpus first: `examples/<name>.arb` beside
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
   as a dependency, `amber check` that parses a script and reports syntax
   errors, the example corpus as the specification, and a test that every
   example parses.
1. **Front end**: `syntax` — amber's AST, the rejection list, the `tap`
   rewrite, with the checker's messages tested.
2. **Evaluator core**: literals, variables, operators, control flow, methods,
   blocks, lambdas, exceptions, `puts`/`print`/`p`/`warn`. `amber script.arb`
   runs scripts without classes.
3. **Standard library**: String, Integer, Float, Symbol, Array, Hash, Set,
   Range, Enumerable.
4. **Classes**: typed fields as dynamic messages, `initialize`, readers,
   `tap`, chained assignment, classes as values bound to their name,
   inheritance with overriding, modules as namespaces and mixins.
5. **Protobuf**: `.proto` generation with enums, packages and `reserved`,
   the value envelope, the Go host API, `amber proto`.
6. **Async**: the scheduler, `async`, `await`, `await :all`, the
   serialisation rule.
7. **Ruby adapter**: `parts/ruby/amber`, run against the same corpus.

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
- **Field numbers** follow declaration order; `reserved` keeps a removed
  field's number and name out of use.
- **Classes are values** bound to their name: a second definition with the
  same body is the same class, one with a different body rebinds the name,
  and nothing reopens a class.
- **Only current Ruby syntax**: keyword arguments are keywords, never a
  trailing hash; `=>` stays for hash literals.
- **The standard library follows CRuby's benchmarks** (`benchmarks/ruby/`):
  - `Array` and `Hash` are the Ruby-visible names of amber's lists and maps,
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
  - `Hash.new` takes a default value or a default block; a block that
    stores into the hash raises, as any mutation does.
  - Left out: `Struct` (its members are untyped and its point is setters;
    amber classes declare typed fields), `Rational` and `Complex` (with
    their literals), and `RUBY_VERSION`.
