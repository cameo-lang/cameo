# cameo and Ruby

cameo runs a subset of Ruby, with Ruby's syntax and, where cameo accepts a
construct, Ruby's behaviour, CRuby 3.4's where 3.3 differs. Everything cameo
accepts parses as Ruby. Where cameo differs it does so in one of three ways:

- **a different meaning**, almost all of it because every value is immutable;
- **rejected before the script runs**: the checker names the construct and,
  where there is one, the alternative;
- **a missing builtin**, which raises `NameError` or `NoMethodError` when the
  script reaches it (`Proc#curry` raises `NotImplementedError`, and
  `Integer()` with a base `ArgumentError`).

This page lists each, as of v0.1. The [design document](design/cameo.md)
says why; its *Decisions taken* records the choices made explicitly.

## How compatible, measured

**CRuby's benchmark suite.** [benchmarks/ruby](../benchmarks/ruby) holds the
1318 cases of CRuby's `benchmark/` that cameo may legally carry. 852 of them
run under cameo with Ruby's output. The others, by the first thing that stops
each:

| Cases | Reason |
|---|---|
| 52 | metaprogramming: `eval`, `define_method`, `class_eval`, `instance_variable_set` |
| 61 | instance variables |
| 59 | `attr_*` accessors |
| 58 | mutation of something other than a local variable |
| 33 | GC and VM introspection |
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
| 1 each | class variables, `String#tr` with a hash |

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
  while`, `loop` and `break` with a value, `next`, `redo`, `retry`, `catch` and
  `throw`, safe navigation `&.`, `BEGIN`.
- **Methods**: positional, optional, `*rest`, keyword, `**kwrest` and
  `&block` parameters; endless `def`; argument forwarding (`...`, `*`, `**`,
  `&`); `yield` and `block_given?`; a second `def` of a name replaces the
  first.
- **Blocks and procs**: `{ }` and `do … end`, numbered parameters (`_1`),
  `->` lambdas, `proc`, `&:sym`, `then` and `tap`.
- **Classes and modules**: single inheritance, overriding and `super`;
  `module` as a namespace and a mixin through `include` and `extend`;
  `Comparable`, and `Enumerable` for a class that defines `each`;
  bang methods a class defines, `def save!`, which return their result
  and rebind nothing;
  `method_missing` and `respond_to_missing?`; `send`, `public_send` and
  `__send__`, which refuse a mutating method as a call
  does; `Class.new(Base) { … }`;
  `def self.x` in a class body.
- **Exceptions**: `begin`/`rescue`/`else`/`ensure`, `rescue => e`,
  `raise`, `retry`, `exit` and `abort`, which raise SystemExit and end the
  command with its status, and the exception classes of Ruby's core.
- **Core classes**: Integer, Float, String, Symbol, Array, Hash, Set (as in
  3.4, without `require`), Range, Regexp and MatchData, Time, Math,
  Enumerator and `lazy`, Comparable and Enumerable, with most of their
  methods. `format` and `String#%`, `pack` and `unpack`, `rand` and `srand`,
  `Integer()`, `Float()`, `String()` and `Array()`,
  `Process.clock_gettime`.
- **Libraries**: `require` of `set`, `time`, `pp`, `pathname`, `cgi`,
  `json` and `benchmark`, which cameo builds in. `cgi` has what Ruby's `cgi/escape` and `cgi/util`
  do without I/O: `escape` and `unescape`, the URI-component pair,
  `escapeHTML` and `unescapeHTML`, `escapeElement` and `unescapeElement`,
  `rfc1123_date` and `pretty`, with their snake_case names. A Symbol
  passes where these take a String; Ruby raises a TypeError. `json` is
  Ruby 3.4's json 2.9.1: `JSON.parse` (with `symbolize_names:`,
  `allow_nan:` and `max_nesting:`), `parse!`, `load`, `generate`, `dump`,
  `pretty_generate` and `JSON()`, with the generator's options, `to_json`
  on every value, and `JSON::ParserError`, `NestingError` and
  `GeneratorError`. Output, parsed values and error messages are Ruby's.
  `benchmark` is Ruby 3.4's benchmark 0.4.0: `Benchmark.measure`,
  `realtime`, `bm`, `bmbm` and `benchmark`, and `Benchmark::Tms` with its
  readers, `format`, `to_a`, `to_h`, arithmetic and `add`. The times are
  read inside the script, so the interpreter's start is not in them; the
  CPU times are the whole process's, the work of `async` calls included.
- **Output**: `puts`, `print`, `p`, `pp`, `warn`.

## Different in cameo

**Values are immutable.** Nothing changes in place, and no call changes or
rebinds its receiver:

- A call of one of Ruby's mutators on a value of the kind it changes
  raises a `CameoError` that names the non-mutating form: `a << 1` and
  `a.push(1)` on an array ("use +, or append"), `s << "x"` and
  `s.setbyte(0, 65)` on a string, `h.store(k, v)`, `h.delete(k)` and
  `h.update(o)` on a hash, `st.add(x)` and `st.merge(o)` on a set. The
  test `src/eval/mutation_test.go` lists every one. With `cameo
  --ignore-mutation-errors` (or `Program.IgnoreMutationErrors` from Go)
  the call instead returns what Ruby's returns, computed on a copy, and
  the receiver keeps its value: `a.pop` is the last element, and `a` is
  as it was.
- A mutator whose value is assigned back to its receiver rebinds it:
  `a = a << 1`, `a <<= 1` and `self.xs = self.xs << x` in `initialize`
  return what Ruby's returns, computed on a copy, and the assignment binds
  it. Another variable that held the old value keeps it.
- A bang method of Ruby's core (`compact!`, `sort!`, `gsub!`, `merge!`)
  is refused before the script runs: "This is not supported, use x =
  x.compact instead". A bang method a class defines is an ordinary call.
- `force_encoding` returns the string relabelled and leaves the receiver,
  since nothing else reinterprets bytes. A call whose value the script
  drops, a statement on its own, raises, since it would do nothing.
- The same names on a value they do not change are ordinary calls:
  `Integer#<<` shifts, and a class's own `push` is its own method.
- Assignment makes the copy and rebinds the local: `a += [1]`, `a.x = 1`,
  `a.x.y = 1` and `h[k] = v` rebind `a` or `h` to a copy with the change.
  The chain must start at a local variable, or at `self` inside
  `initialize`.
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
- A `throw` ends the `catch` whose tag is equal to its own, where Ruby
  wants the same object: `catch("a") { throw "a" }` returns nil. The
  Object a `catch` without a tag makes is still a tag of its own.
- `rand` and `srand` use Go's generator, so a seeded sequence is not
  Ruby's.
- `Time` is immutable: `utc` and `localtime` return a new time, and
  `subsec` is a Float, since `Rational` is left out.
- A String is UTF-8, ASCII-8BIT (binary) or US-ASCII; other encodings do not
  exist.
- Regular expressions use Ruby's syntax on cameo's own engine; lookbehind
  and backreferences are rejected with "This regexp is not supported in
  Cameo".
- `JSON.generate` writes an instance of a cameo class as an object of its
  fields, by name in declaration order, where Ruby writes the string of
  `to_s`. A class that defines `to_json` writes itself, as in Ruby, and
  receives the generator's options as a Hash with `depth:` instead of a
  `JSON::State`; passing it on to `to_json` nests the same way.
- `Benchmark::Tms#add!` does not exist: it changes its receiver. Use
  `t = t.add { ... }`. `p` of a `Benchmark::Tms` shows no address.
- A Symbol becomes a String where a value leaves cameo, in a protobuf
  message or an `async` call, unless an `enum` field declares it.
- Ruby's version constants describe cameo. A script tells cameo from CRuby
  by `RUBY_ENGINE`, and cannot assign any of them at the top level:

  | Constant | In cameo |
  |---|---|
  | `RUBY_ENGINE` | `"cameo"` |
  | `RUBY_ENGINE_VERSION` | cameo's version, as `cameo version` prints it |
  | `RUBY_VERSION` | `"3.4.0"`: the Ruby cameo follows, 3.4 as released |
  | `RUBY_PATCHLEVEL` | `0`, Ruby 3.4.0's; cameo takes no CRuby patch release |
  | `RUBY_PLATFORM` | Ruby's name for the platform, as `x86_64-linux`, `aarch64-linux`, `arm64-darwin` or `x64-mingw-ucrt`, without the OS version CRuby appends on macOS |
  | `RUBY_RELEASE_DATE` | the date of the commit cameo was built from, or `"unknown"` |
  | `RUBY_REVISION` | the hash of that commit, or `"unknown"` |
  | `RUBY_DESCRIPTION` | the build in CRuby's form: `cameo 0.2.3 (2026-10-20 revision 0123456789) +ruby 3.4.0 [x86_64-linux]` |
  | `RUBY_COPYRIGHT` | `"cameo - Copyright (C) 2026 the cameo authors (MIT License)"` |

  The commit is known when the binary was built with `go build` in a git
  checkout; a binary from `go install ...@v0.2.3` knows its version but no
  commit.

## Rejected before the script runs

The checker rejects these by name, with the line:

| Construct | Instead |
|---|---|
| instance variables, `@x` | `self.x = v` in `initialize`; read `x` or `self.x` |
| class variables, `@@x`; global variables, `$x`, including `$stdout`, `$!` and `$~` | a constant, a field or a local; `rescue => e`; the MatchData of `match` |
| setters, `def x=(v)`; `attr_reader`, `attr_writer`, `attr_accessor` | field declarations, which define readers, and `with` |
| `private`, `protected`, `public`, `module_function` | none: every method is public |
| `eval`, `instance_eval`, `class_eval`, `define_method`, `instance_variable_get` and `_set`, `undef` | a direct call or a `def` |
| `def obj.x`, `class << self` | `def self.x` in a class body |
| a bang method of Ruby's core, such as `a.sort!` | the copying form, assigned: `a = a.sort` |
| nested multiple assignment, `a, (b, c) = …` | separate assignments |
| `require_relative`, `load` | one file per script |
| backticks, `fork` | none; `fork`'s message names `async` |
| rational and imaginary literals, `1r` and `2i` | none |
| assignment to a scoped constant, `A::B = 1` | assignment inside `A` |

## Missing builtins

These are absent, and a script that uses one fails when it reaches it:

- **Left out by design**: `Struct` and `Data` (classes declare typed
  fields), `Rational` and `Complex`, `Marshal` (instances are protobuf
  messages), `ObjectSpace`, `GC`, `ENV`, `Module.new`,
  refinements, `binding`, `method` and `Proc#curry`.
- **No I/O beyond standard output and error**: `File`, `IO`, `Dir`,
  `gets`, `STDIN`, `STDOUT`, `STDERR`, `$stdout`.
- **No threads or processes**: `Thread`, `Fiber`, `Mutex`, `sleep`,
  `system`, `exec`, `spawn`; cameo has `async` and `await` instead.
- **Not there yet**: `at_exit`, `exit!`,
  `printf` (`format` exists), `__method__`, `__LINE__`, `Enumerator.new`,
  `Integer()` with a base, `Random` as a class (`rand` and `srand` exist),
  and `autoload`.
- **Libraries**: `require` of any library but the seven above, such as
  `yaml`, raises a `LoadError`.

## Only in cameo

- Typed fields, with every class a protobuf message: `to_proto`,
  `from_proto`, and `Point.proto` for its `.proto` text.
- `with`, which returns a copy with some fields changed.
- The Cameo module, which a script includes: `async receiver, :method,
  args…` runs a method concurrently and returns its promise, an
  Integer; `await` collects a promise's result, or a list's, and `await
  :all` every one the scope started; `promise`, `resolve` and `fail` make
  and settle a promise by hand, and a Go host settles the promises its
  methods return when their I/O is done. `Cameo::Runner` is the same
  module under the name that Ruby has too: `ruby/lib/cameo/runner.rb`
  gives CRuby `async`, `await` and promises on threads, so a script that
  starts with `require "cameo/runner"` and whose classes include
  `Cameo::Runner` runs on both. The top level refuses `Cameo::Runner` on
  both; `include Cameo` at the top level is cameo's alone.
- The Go host API: a Go program loads a script, calls into it, and passes
  protobuf messages both ways. A host can give the script a module of its
  own, whose methods are Go functions, and native classes: classes whose
  fields are a generated protobuf message and whose methods are Go
  functions (`cameo.Register`).
