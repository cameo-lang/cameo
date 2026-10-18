# cameo and Ruby

cameo runs a subset of Ruby, with Ruby's syntax and, where cameo accepts a
construct, Ruby's behaviour, CRuby 3.4's where 3.3 differs. Everything cameo
accepts parses as Ruby. Where cameo differs it does so in one of three ways:

- **a different meaning**, almost all of it because every value is immutable;
- **rejected before the script runs**: the checker names the construct and,
  where there is one, the alternative;
- **a missing builtin**, which raises `NameError` or `NoMethodError` when the
  script reaches it.

This page lists each, as of v0.1. The [design document](design/cameo.md)
says why; its *Decisions taken* records the choices made explicitly.

## How compatible, measured

**CRuby's benchmark suite.** [benchmarks/ruby](../benchmarks/ruby) holds the
1318 cases of CRuby's `benchmark/` that cameo may legally carry. 824 of them
run under cameo with Ruby's output. The others, by the first thing that stops
each:

| Cases | Reason |
|---|---|
| 80 | metaprogramming: `send`, `eval`, `define_method`, `class_eval`, `instance_variable_set` |
| 61 | instance variables |
| 59 | `attr_*` accessors |
| 58 | mutation of something other than a local variable |
| 32 | GC and VM introspection |
| 32 | `Module.new` |
| 31 | the file system |
| 24 | threads and fibers |
| 24 | `Rational` and `Complex` |
| 22 | `require` of a library cameo does not build in |
| 18 | reopening a builtin class |
| 10 | `Struct` |
| 9 | singleton methods outside a class |
| 9 | global variables |
| 8 | `Marshal` |
| 6 | `ruby2_keywords` |
| 5 | object identity |
| 3 | `ENV` |
| 1 each | `RUBY_VERSION`, class variables, `String#tr` with a hash |

**ruby/spec.** [specs](../specs) sorts all 3841 spec files of
[ruby/spec](https://github.com/ruby/spec) by what cameo's checker accepts:
1403 are accepted. None has been run, so this counts the constructs a spec
uses, not whether its examples pass. Most of the rest require a library cameo
does not build in (1006), or use an instance variable (852), mostly in
mspec's own `before` blocks.

## The same as in Ruby

- **Literals**: `nil`, `true`, `false`, integers of any size, floats,
  strings with interpolation and heredocs, symbols, arrays, hashes, ranges,
  regular expressions, `%w[]` and `%i[]`.
- **Control flow**: `if`/`unless`/`elsif`, modifiers, `case`/`when`,
  pattern matching with `case`/`in`, `while`/`until` and `begin … end
  while`, `loop` and `break` with a value, `next`, `retry`, safe
  navigation `&.`, `BEGIN`.
- **Methods**: positional, optional, `*rest`, keyword, `**kwrest` and
  `&block` parameters; endless `def`; argument forwarding (`...`, `*`, `**`,
  `&`); `yield` and `block_given?`; a second `def` of a name replaces the
  first.
- **Blocks and procs**: `{ }` and `do … end`, numbered parameters (`_1`),
  `->` lambdas, `proc`, `&:sym`, `then` and `tap`.
- **Classes and modules**: single inheritance, overriding and `super`;
  `module` as a namespace and a mixin through `include` and `extend`;
  `Comparable`, and `Enumerable` for a class that defines `each`;
  `method_missing` and `respond_to_missing?`; `Class.new(Base) { … }`;
  `def self.x` in a class body.
- **Exceptions**: `begin`/`rescue`/`else`/`ensure`, `rescue => e`,
  `raise`, `retry`, and the exception classes of Ruby's core.
- **Core classes**: Integer, Float, String, Symbol, Array, Hash, Set (as in
  3.4, without `require`), Range, Regexp and MatchData, Time, Math,
  Enumerator and `lazy`, Comparable and Enumerable, with most of their
  methods. `format` and `String#%`, `pack` and `unpack`, `rand` and `srand`,
  `Integer()`, `Float()`, `String()` and `Array()`,
  `Process.clock_gettime`.
- **Libraries**: `require` of `set`, `time`, `pp`, `pathname` and `cgi`,
  which cameo builds in.
- **Output**: `puts`, `print`, `p`, `pp`, `warn`.

## Different in cameo

**Values are immutable.** Nothing changes in place, so what would change a
value in Ruby makes a changed copy:

- A mutating call on a local variable rebinds it: `s << "x"`, `a.push(1)`,
  `a.map! { … }` and `h.merge!(other)` leave the local holding the changed
  copy, and any other reference holds what it held before.
- `a.x = 1`, `a.x.y = 1` and `h[k] = v` rebind the local `a` or `h` to a
  copy with the change. The chain must start at a local variable, or at
  `self` inside `initialize`.
- `frozen?` is true for everything; `freeze` and `dup` change nothing.
- Equality is structural: two arrays, hashes or instances with equal
  contents are `==` and hash alike. There is no object identity, so
  `equal?` and `object_id` do not exist, and `compare_by_identity` changes
  nothing.

**Classes declare typed fields**, and every instance is a protobuf message:

```ruby
class Point
  int32 :x, :y
  string :label, default: ""

  def initialize(x, y)
    self.x = x
    self.y = y
  end
end

p = Point.new(1, 2)
q = p.with(x: 3)            # a copy; p is unchanged
```

- A field's type is one of proto3's scalar types (`int32`, `uint64`,
  `double`, `bool`, `string`, `bytes` and the rest), `int` and `float` for
  Ruby's Integer and Float, `bignum` for an Integer of any size, or `list`,
  `set`, `map`, `ref` and `enum`. A value outside its type raises a
  `TypeError` or a `RangeError`.
- `initialize` sets a field with `self.x = v`; a method reads it with `x`
  or `self.x`. There are no instance variables and no setters: `with` makes
  the changed copy.
- A class without `initialize` takes its fields as keywords:
  `Point.new(x: 1, y: 2)`.
- A class is a value bound to its name. Evaluating the same `class X … end`
  again gives the same class; a different body makes a new class and
  rebinds `X`, leaving the old one to its instances. Nothing reopens a
  class once its body has run, and a builtin class cannot be reopened.

**Smaller differences:**

- Hash and array literals, and errors, print as CRuby 3.4 prints them:
  `{x: 1}`, not 3.3's `{:x=>1}`.
- A bare `Object` prints as `#<Object>`, without an address.
- `rand` and `srand` use Go's generator, so a seeded sequence is not
  Ruby's.
- `Time` is immutable: `utc` and `localtime` return a new time, and
  `subsec` is a Float, since `Rational` is left out.
- A String is UTF-8, ASCII-8BIT (binary) or US-ASCII; other encodings do not
  exist.
- Regular expressions use Ruby's syntax on cameo's own engine; lookbehind
  and backreferences are rejected with "This regexp is not supported in
  Cameo".
- A Symbol becomes a String where a value leaves cameo, in a protobuf
  message or an `async` call, unless an `enum` field declares it.

## Rejected before the script runs

The checker rejects these by name, with the line:

| Construct | Instead |
|---|---|
| instance variables, `@x` | `self.x = v` in `initialize`; read `x` or `self.x` |
| class variables, `@@x`; global variables, `$x`, including `$stdout`, `$!` and `$~` | a constant, a field or a local; `rescue => e`; the MatchData of `match` |
| setters, `def x=(v)`; `attr_reader`, `attr_writer`, `attr_accessor` | field declarations, which define readers, and `with` |
| `private`, `protected`, `public`, `module_function` | none: every method is public |
| `send`, `public_send`, `eval`, `instance_eval`, `class_eval`, `define_method`, `instance_variable_get` and `_set`, `undef` | a direct call or a `def` |
| `def obj.x`, `class << self` | `def self.x` in a class body |
| a mutating or bang method on anything but a local, such as `[3, 1].sort!` or `obj.list << x` | the non-mutating form: `sort`, `+`, `append`, `with` |
| nested multiple assignment, `a, (b, c) = …` | separate assignments |
| `require_relative`, `load` | one file per script |
| backticks, `system`, `exec`, `spawn`, `fork` | none |
| rational and imaginary literals, `1r` and `2i` | none |
| assignment to a scoped constant, `A::B = 1` | assignment inside `A` |

## Missing builtins

These are absent, and a script that uses one fails when it reaches it:

- **Left out by design**: `Struct` and `Data` (classes declare typed
  fields), `Rational` and `Complex`, `Marshal` (instances are protobuf
  messages), `ObjectSpace`, `GC`, `ENV`, `RUBY_VERSION`, `Module.new`,
  refinements, `binding`, `method` and `Proc#curry`.
- **No I/O beyond standard output and error**: `File`, `IO`, `Dir`,
  `gets`, `STDIN`, `STDOUT`, `STDERR`, `$stdout`.
- **No threads or processes**: `Thread`, `Fiber`, `Mutex`, `sleep`; cameo
  has `async` and `await` instead.
- **Not there yet**: `exit`, `at_exit`, `catch` and `throw`, `redo`,
  `printf` (`format` exists), `__method__`, `__LINE__`, `Enumerator.new`,
  `Integer()` with a base, `Random` as a class (`rand` and `srand` exist),
  and `autoload`.
- **Libraries**: `require` of any library but the five above, such as
  `json`, raises a `LoadError`.

## Only in cameo

- Typed fields, with every class a protobuf message: `to_proto`,
  `from_proto`, and `Point.proto` for its `.proto` text.
- `with`, which returns a copy with some fields changed.
- `async receiver, :method, args…` runs a method on its own goroutine;
  `await` collects the result, and `await :all` every one.
- The Go host API: a Go program loads a script, calls into it, and passes
  protobuf messages both ways. A host can give the script a module of its
  own, whose methods are Go functions, and native classes: classes whose
  fields are a generated protobuf message and whose methods are Go
  functions (`cameo.Register`).
