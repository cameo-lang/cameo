## Ruby vs cameo {#ch-ruby}

::: source
from `doc/gh_pages/ruby.html`
:::

Everything cameo accepts parses as Ruby, and where cameo accepts a construct
it behaves as CRuby 3.4 does. The differences are below, each with the Ruby
you would write and the cameo way.

1.  [The same as in Ruby](#ruby-same)
2.  [Values are immutable](#ruby-immutable)
3.  [Typed fields instead of instance variables](#ruby-fields)
4.  [Strings are UTF-8 or binary](#ruby-strings)
5.  [What the checker rejects](#ruby-rejected)
6.  [Missing builtins](#ruby-missing)
7.  [Smaller differences](#ruby-smaller)
8.  [Only in cameo: with, tail calls, the IR, async, await and promises,
    protobuf](#ruby-only)

cameo differs from Ruby in three ways. A construct has a different meaning,
almost always because values are immutable. The checker rejects a construct
before the script runs, and names what to write instead. Or a builtin is
missing, which raises an exception, mostly `NameError` or `NoMethodError`,
when the script reaches it. The full list is
[doc/cameo-vs-ruby.md](#ch-reference-cameo-vs-ruby).

### The same as in Ruby {#ruby-same}

-   **Literals**: `nil`, booleans, integers of any size, floats, strings with
    interpolation and heredocs, symbols, arrays, hashes, ranges, regular
    expressions, `%w[]` and `%i[]`.
-   **Control flow**: `if`, `unless`, modifiers, `case`/`when`, pattern
    matching with `case`/`in`, `while`, `until`, `loop`, `break` with a value,
    `next`, `redo`, `catch` and `throw`, `&.`, `exit` and `at_exit`.
-   **Methods and blocks**: positional, optional, rest, keyword and block
    parameters; endless `def`; argument forwarding; `yield`; lambdas, procs,
    `&:sym`, numbered parameters.
-   **Classes and modules**: single inheritance and `super`; modules as
    namespaces and mixins through `include` and `extend`; `Comparable`,
    `Enumerable`, `method_missing`.
-   **Exceptions**: `begin`/`rescue`/`else`/`ensure`, `raise`, `retry`, and
    Ruby's exception classes.
-   **The core classes**: Integer, Float, String, Symbol, Array, Hash, Set,
    Range, Regexp, MatchData, Time, Math, Enumerator and `lazy`, with most of
    their methods; `format`, `pack`, `rand` and `Random`.
-   **The built-in libraries**: `require` of `set`, `time`, `pp`, `pathname`,
    `cgi`, `json`, `benchmark`, `uri`, `date`, `digest`, `base64`,
    `securerandom`, `csv`, `marshal`, `shellwords`, `ipaddr`, `zlib`, `prime`,
    `tsort`, `abbrev`, `singleton`, `forwardable`, `delegate`, `logger`,
    `stringio`, `strscan`, `optparse`, `getoptlong`, `matrix`, `bigdecimal`
    and `unicode_normalize`, and cameo's own `minispec`, minitest's assertions
    in `describe` and `it` blocks. [doc/stdlib.md](#ch-reference-stdlib) says
    what each covers.

This script prints the same under `ruby` 3.4 and under `cameo`:

``` {.ruby .cameo}
def describe(shape)
  case shape
  in { kind: :circle, r: } then "circle, area #{(Math::PI * r**2).round(2)}"
  in { kind: :square, side: } then "square, area #{side * side}"
  in [x, y] then "point at #{x},#{y}"
  end
end

shapes = [{ kind: :circle, r: 1.5 }, { kind: :square, side: 3 }, [4, 5]]
shapes.each_with_index { |s, i| puts "#{i + 1}. #{describe(s)}" }

words = %w[pear apple fig apple pear apple]
p words.tally.max_by { _2 }
p words.uniq.sort_by(&:length).group_by(&:size).to_a

begin
  Integer("12x")
rescue ArgumentError => e
  puts "rescued #{e.class}: #{e.message}"
ensure
  puts "done"
end
```

``` output
1. circle, area 7.07
2. square, area 9
3. point at 4,5
["apple", 3]
[[3, ["fig"]], [4, ["pear"]], [5, ["apple"]]]
rescued ArgumentError: invalid value for Integer(): "12x"
done
```

Of the 1318 cases in CRuby's benchmark suite that cameo may carry, 781 run
under cameo with Ruby's output. What stops most of the rest is
metaprogramming, instance variables, `attr_*` accessors and mutation of
something other than a local variable, which the sections below cover.

### Values are immutable {#ruby-immutable}

Nothing changes in place, and no call rebinds its receiver. A call of one of
Ruby's mutators (`<<`, `push`, `delete`, `store`, `add`) on a value it would
change raises a `CameoError` that names the non-mutating form;
`cameo --ignore-mutation-errors` makes it return Ruby's result instead and
leaves the receiver as it was. Assignment makes the copy and rebinds the
local: `a = a << 3`, `a += [3]`, `h[k] = v`, `a.x = 1`. Another variable that
held the old value keeps it.

::: pair
::: side
##### Ruby

``` ruby
a = [1, 2]
b = a
a << 3
p a, b
```

``` output
[1, 2, 3]
[1, 2, 3]
```
:::

::: side
##### cameo

``` {.ruby .cameo}
a = [1, 2]
b = a
begin
  a << 3
rescue CameoError => e
  puts e.message
end
a += [3]
p a, b
```

``` output
cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (see https://15b.it/mut)
[1, 2, 3]
[1, 2]
```
:::
:::

A bang method of Ruby's core is refused before the script runs, with the
copying form to write instead.

::: pair
::: side
##### Ruby

``` ruby
a = [3, 1, 2]
a.sort!
p a
```

``` output
[1, 2, 3]
```
:::

::: side
##### cameo

``` {.ruby .cameo .rejected}
a = [3, 1, 2]
a.sort!
p a
```

``` {.output .error}
script.cameo:2: This is not supported, use a = a.sort instead (see https://15b.it/mut)
```

``` {.ruby .cameo}
a = [3, 1, 2]
a = a.sort
p a
```

``` output
[1, 2, 3]
```
:::
:::

Equality is structural: two arrays, hashes or instances with equal contents
are `==` and hash alike. There is no object identity, so `equal?` and
`object_id` do not exist. `frozen?` is true for every value, and `freeze` and
`dup` change nothing.

::: why
**Why #1** Values are immutable
:::

#### A block captures values {#ruby-blocks}

A block, a proc or a lambda keeps the values its variables have when it is
made. It cannot assign to a variable of the scope around it, and it does not
see a later assignment there. This is unlike Ruby, where a block shares the
variables themselves. The checker refuses the assignment and names what to
write instead.

::: pair
::: side
##### Ruby

``` ruby
total = 0
[1, 2, 3].each { |n| total += n }
p total

base = 10
add = ->(n) { n + base }
base = 20
p add.(1)
```

``` output
6
21
```
:::

::: side
##### cameo

``` {.ruby .cameo .rejected}
total = 0
[1, 2, 3].each { |n| total += n }
p total
```

``` {.output .error}
script.cameo:2: a block cannot rebind total, a variable of the scope around it: a block captures the values its variables have when it is made; bind a new name, or use reduce, each_with_object or while (see https://15b.it/blk)
```

``` {.ruby .cameo}
total = [1, 2, 3].reduce(0) { |sum, n| sum + n }
p total

base = 10
add = ->(n) { n + base }
base = 20
p add.(1)
```

``` output
6
11
```
:::
:::

::: why
**Why #2** A block captures values
:::

### Typed fields instead of instance variables {#ruby-fields}

A class declares its fields with their types. Each field gets a reader. There
are no instance variables and no setters: `initialize` is the one place a
field is set, with `self.x = v`, and `with` makes a changed copy everywhere
else.

::: pair
::: side
##### Ruby

``` ruby
class Counter
  attr_reader :count

  def initialize
    @count = 0
  end

  def increment
    @count += 1
    self
  end
end

c = Counter.new
c.increment.increment
p c.count
```

``` output
2
```
:::

::: side
##### cameo

``` {.ruby .cameo}
class Counter
  int :count, default: 0

  def increment
    with(count: count + 1)
  end
end

c = Counter.new
c = c.increment.increment
p c.count
```

``` output
2
```
:::
:::

A field's type is one of proto3's scalar types (`int32`, `uint64`, `double`,
`bool`, `string`, `bytes` and the rest), `int` and `float` for Ruby's Integer
and Float, `bignum` for an Integer of any size, or `list`, `set`, `map`, `ref`
and `enum`. A value of the wrong type raises `TypeError`, and one out of range
`RangeError`. A class without `initialize` takes its fields as keywords.

``` {.ruby .cameo}
class Point
  int32 :x, :y
  string :label, default: ""
end

pt = Point.new(x: 1, y: 2)
p pt
p pt == Point.new(x: 1, y: 2, label: "")

begin
  Point.new(x: "one")
rescue TypeError => e
  puts e.message
end
```

``` output
Point(x: 1, y: 2, label: "")
true
field x of Point takes an int32, not an instance of String
```

Assignment through a chain of readers that starts at a local variable is a
copy too: `line.to.x = 30` rebinds `line` to a copy whose `to` is a copy with
`x` set.

``` {.ruby .cameo}
class Point
  int :x, :y
end

class Line
  ref :from, Point
  ref :to, Point
end

line = Line.new(from: Point.new(x: 0, y: 0), to: Point.new(x: 3, y: 4))
before = line
line.to.x = 30
p line.to, before.to
```

``` output
Point(x: 30, y: 4)
Point(x: 3, y: 4)
```

A class is a value bound to its name. Evaluating the same `class X … end`
again gives the same class; a different body reopens a copy of the class and
rebinds `X` to it. Instances made before, and classes that inherit from it,
keep the class as it was and do not see what the reopening adds. A builtin
class cannot be reopened.

::: why
**Why #3** Typed fields instead of instance variables
:::

### Strings are UTF-8 or binary {#ruby-strings}

A String is UTF-8, US-ASCII or binary; Ruby's other encodings do not exist. A
UTF-8 String holds whole characters alone, so none is invalid, where Ruby lets
a String hold bytes that do not fit its encoding. Bytes that are not UTF-8 are
a binary String, written `b"\xFF"`; Ruby reads that as a call of a method `b`.
The plain literal `"\xFF"` is refused before the script runs:

``` {.ruby .cameo .rejected}
s = "\xFF"
```

``` {.output .error}
script.cameo:1: a string literal with bytes that are not UTF-8 is a binary string and is written b"..." (see https://15b.it/inv)
```

A binary String has bytes and no characters. The methods on bytes, comparison,
`+`, `[]` and `length` work as in Ruby; a method that reads characters, such
as `upcase`, `strip` or `split`, raises, where Ruby reads the bytes as ASCII.
`force_encoding("UTF-8")` says what the bytes are, and raises when they are
not UTF-8. `getbyte` goes the other way: it reads a byte of a binary String or
of one that is ASCII alone, and raises for a UTF-8 String with characters
beyond ASCII, where Ruby returns a part of a character; `s.b.getbyte(i)` says
the bytes are meant.

Binary joined with anything is binary: `+`, `<<`, `concat` and `Array#join`
return a binary String as soon as one part is binary, where Ruby returns UTF-8
when the binary part is ASCII alone and raises when both hold other bytes.
What makes text for a reader refuses a binary String: an interpolation, and
`format`, `sprintf` and `%`.

``` {.ruby .cameo}
bytes = b"ab"
p ("x" + bytes).encoding, ["x", bytes].join.encoding

begin
  "v=#{bytes}"
rescue Encoding::CompatibilityError => e
  puts e.message
end

puts "v=#{bytes.force_encoding("UTF-8")}"
```

``` output
#<Encoding:BINARY (ASCII-8BIT)>
#<Encoding:BINARY (ASCII-8BIT)>
an interpolation makes text, and a binary string has bytes alone: say what they are with force_encoding first, or join bytes with + (see https://15b.it/bin)
v=ab
```

``` {.ruby .cameo}
bytes = b"\xFFab"
p bytes.encoding
p bytes.bytesize
p bytes + "c"

begin
  bytes.upcase
rescue Encoding::CompatibilityError => e
  puts e.message
end

begin
  bytes.force_encoding("UTF-8")
rescue Encoding::InvalidByteSequenceError => e
  puts e.class
end

p b"caf\xC3\xA9".force_encoding("UTF-8").upcase
```

``` output
#<Encoding:BINARY (ASCII-8BIT)>
3
"\xFFabc"
upcase reads characters, and a binary string has bytes alone: say what they are with force_encoding first (see https://15b.it/bin)
Encoding::InvalidByteSequenceError
"CAFÉ"
```

::: why
**Why #4** Strings are UTF-8 or binary
:::

### What the checker rejects {#ruby-rejected}

cameo checks a whole script before it runs any of it, and reports every
construct it leaves out, with its line. `cameo check script.cameo` runs the
check alone. Each refusal ends with the page of its code, which says what
cameo leaves out, why, and what to write instead:
[ivar](https://cameo-lang.github.io/cameo/compatibility/ivar.html),
[mut](https://cameo-lang.github.io/cameo/compatibility/mut.html),
[blk](https://cameo-lang.github.io/cameo/compatibility/blk.html) and the
[others](#compatibility-not-expected-to-work).

``` {.ruby .cameo .rejected}
class Account
  attr_reader :balance

  def initialize
    @balance = 0
  end

  define_method(:deposit) { |n| add(n) }
end

$total = 0
```

``` {.output .error}
script.cameo:2: attr_reader is not supported: a field declaration defines its reader, and there are no setters (see https://15b.it/attr)
script.cameo:5: @balance = …: cameo has no instance variables; initialize sets a field with self.balance = … (see https://15b.it/ivar)
script.cameo:8: define_method is not supported; use def (see https://15b.it/meta)
script.cameo:11: global variables are not supported (see https://15b.it/glob)
```

::: rejected
**Rejected:** instance variables, `@x`

**Write instead:** `self.x = v` in `initialize`; read `x` or `self.x`
:::

::: why
**Why #5** instance variables, \@x
:::

::: rejected
**Rejected:** class variables `@@x`; global variables `$x`, including `$!` and
`$~`

**Write instead:** a constant, a field or a local; `rescue => e`; the
MatchData that `match` returns. `$stdout` and `$stderr` exist: they are
`STDOUT` and `STDERR`, with `puts`, `print`, `write`, `flush` and `sync`
:::

::: why
**Why #6** class variables @@x; global variables \$x, including \$! and \$\~
:::

::: rejected
**Rejected:** setters, `def x=(v)`; `attr_reader`, `attr_writer`,
`attr_accessor`

**Write instead:** field declarations, which define readers, and `with`
:::

::: why
**Why #7** setters, def x=(v); attr_reader, attr_writer, attr_accessor
:::

::: rejected
**Rejected:** `self.x = v` outside `initialize`

**Write instead:** `with(x: v)`, which returns the copy
:::

::: why
**Why #8** self.x = v outside initialize
:::

::: rejected
**Rejected:** `private`, `protected`, `public`, `module_function`

**Write instead:** none: every method is public
:::

::: why
**Why #9** private, protected, public, module_function
:::

::: rejected
**Rejected:** `eval`, `instance_eval`, `class_eval`, `define_method`,
`instance_variable_get` and `_set`, `undef`

**Write instead:** a direct call or a `def`
:::

::: why
**Why #10** eval, instance_eval, class_eval, define_method,
instance_variable_get and …
:::

::: rejected
**Rejected:** `def obj.x`, `class << self`

**Write instead:** `def self.x` in a class body
:::

::: why
**Why #11** def obj.x, class &lt;&lt; self
:::

::: rejected
**Rejected:** a bang method of Ruby's core, such as `a.sort!`

**Write instead:** the copying form, assigned: `a = a.sort`
:::

::: why
**Why #12** a bang method of Ruby's core, such as a.sort!
:::

::: rejected
**Rejected:** nested multiple assignment, `a, (b, c) = …`

**Write instead:** separate assignments
:::

::: why
**Why #13** nested multiple assignment, a, (b, c) = …
:::

::: rejected
**Rejected:** `require_relative`, `load`

**Write instead:** one file per script
:::

::: why
**Why #14** require_relative, load
:::

::: rejected
**Rejected:** `exit!`

**Write instead:** `exit`
:::

::: why
**Why #15** exit!
:::

::: rejected
**Rejected:** backticks, `fork`

**Write instead:** none; a host can give the script a module of Go methods,
and `async` runs a call on a goroutine
:::

::: why
**Why #16** backticks, fork
:::

::: rejected
**Rejected:** rational and imaginary literals, `1r` and `2i`

**Write instead:** none
:::

::: why
**Why #17** rational and imaginary literals, 1r and 2i
:::

::: rejected
**Rejected:** assignment to a scoped constant, `A::B = 1`

**Write instead:** the assignment inside `A`
:::

::: why
**Why #18** assignment to a scoped constant, A::B = 1
:::

::: rejected
**Rejected:** a regular expression that needs backtracking: backreferences,
lookahead and lookbehind, atomic groups, possessive quantifiers

**Write instead:** a pattern without them; cameo's regular expressions run in
linear time
:::

::: why
**Why #19** a regular expression that needs backtracking: backreferences,
lookahead and …
:::

::: why
**Why #20** the checker's refusals as a whole
:::

### Missing builtins {#ruby-missing}

These do not exist in cameo. A script that uses one fails when it reaches it,
mostly with `NameError` or `NoMethodError`, and for a library with
`LoadError`.

-   **Left out by design**: `Struct` and `Data` (classes declare typed
    fields), `Rational` and `Complex`, `ObjectSpace`, `GC`, `ENV`,
    `Module.new`, refinements, `binding` beyond the debugger's
    `binding.break`, `method`, `Proc#curry`.

    ::: why
    **Why #21** Left out by design: Struct and Data (classes declare typed
    fields), Rational …
    :::
-   **No I/O unless the host allows it**: a script writes to standard output
    and error. `File.read`, `File.open` and `File.write` exist once the host
    allows files, with `cameo --io=file` or from Go; see [I/O](#ch-io).
    `File`'s path methods (`basename`, `dirname`, `extname`, `join`, `split`,
    `expand_path` and `absolute_path`) always work. `Dir`, `gets`, `STDIN` and
    sockets do not exist.

    ::: why
    **Why #22** No I/O unless the host allows it: a script writes to standard
    output and …
    :::
-   **No threads or processes**: `Thread`, `Fiber`, `Mutex`, `sleep`,
    `system`, `exec`, `spawn`. cameo has `async` and `await` instead.

    ::: why
    **Why #23** No threads or processes: Thread, Fiber, Mutex, sleep, system,
    exec, spawn. …
    :::
-   **Not there yet**: `printf` (`format` exists), `__method__`,
    `Enumerator.new`, `Integer()` with a base, `autoload`.

    ::: why
    **Why #24** Not there yet: printf (format exists), \_\_method\_\_,
    Enumerator.new, Integer() …
    :::
-   **Libraries**: `require` of a library not in the list above, such as
    `resolv`, `yaml` or `net/http`. [doc/stdlib.md](#ch-reference-stdlib) says
    which are planned and which are left out.

    ::: why
    **Why #25** Libraries: require of a library not in the list above, such as
    resolv, yaml …
    :::

::: pair
::: side
##### Ruby

``` ruby
Item = Struct.new(:name, :qty)
item = Item.new("pen", 1)
item.qty += 1
p item.qty
```

``` output
2
```
:::

::: side
##### cameo

``` {.ruby .cameo}
class Item
  string :name
  int :qty
end

item = Item.new(name: "pen", qty: 1)
item.qty += 1
p item.qty
```

``` output
2
```
:::
:::

``` {.ruby .cameo}
begin
  require "resolv"
rescue LoadError => e
  puts e.message
end
```

``` output
cannot load such file -- resolv (see https://15b.it/req)
```

Ruby's version constants describe cameo: `RUBY_ENGINE` is `"cameo"`,
`RUBY_VERSION` is the Ruby cameo follows, 3.4.0, and `RUBY_ENGINE_VERSION` is
cameo's own version. `RUBY_PLATFORM`, `RUBY_DESCRIPTION`, `RUBY_REVISION` and
`RUBY_RELEASE_DATE` describe the build, and `RUBY_PATCHLEVEL` is 0. A script
cannot assign them.

``` {.ruby .cameo}
puts RUBY_ENGINE
puts RUBY_VERSION
puts RUBY_VERSION >= "3.4"
```

``` output
cameo
3.4.0
true
```

### Smaller differences {#ruby-smaller}

-   `throw` ends the `catch` whose tag is equal to its own, where Ruby wants
    the same object, so a String works as a tag.

    ::: why
    **Why #26** throw ends the catch whose tag is equal to its own, where Ruby
    wants the same …
    :::
-   `Time` is immutable: `utc` and `localtime` return a new time and leave the
    receiver as it was. `subsec` is a Float, since `Rational` is left out.

    ::: why
    **Why #27** Time is immutable: utc and localtime return a new time and
    leave the receiver …
    :::
-   Values print as CRuby 3.4 prints them, `{x: 1}`. A bare Object prints as
    `#<Object>`, without an address.

    ::: why
    **Why #28** Values print as CRuby 3.4 prints them, {x: 1}. A bare Object
    prints as …
    :::
-   `__LINE__` is the line its statement starts on.

    ::: why
    **Why #29** \_\_LINE\_\_ is the line its statement starts on.
    :::
-   `JSON.generate` writes an instance of a cameo class as an object of its
    fields, where Ruby writes the string of `to_s`.

    ::: why
    **Why #30** JSON.generate writes an instance of a cameo class as an object
    of its fields, …
    :::
-   `methods` and `instance_methods` return sorted lists.

    ::: why
    **Why #31** methods and instance_methods return sorted lists.
    :::

``` {.ruby .cameo}
p catch("a") { throw "a", 1 }
t = Time.utc(2026, 1, 2, 3, 4, 5)
u = t.localtime("+01:00")
p t.hour, u.hour
```

``` output
1
3
4
```

In Ruby the first line raises `UncaughtThrowError`, and the two hours are 4
and 4: `localtime` changes `t`.

### Only in cameo {#ruby-only}

#### `with`

`value.with(field: v)` returns a copy with the named fields replaced; it is
Ruby's `Data#with`. With a block, the block gets the value and returns a hash
of the changes. On a Hash, `with` merges.

``` {.ruby .cameo}
class Point
  int :x, :y
end

pt = Point.new(x: 1, y: 2)
p pt.with(x: 10)
p pt.with { |q| { y: q.y * 5 } }
p pt
p({ a: 1 }.with(b: 2))
```

``` output
Point(x: 10, y: 2)
Point(x: 1, y: 10)
Point(x: 1, y: 2)
{a: 1, b: 2}
```

#### Tail calls {#ruby-tail}

A call whose value is the value of the method it is in is a tail call. cameo
runs it in place of its caller, which does not stay on the stack, so a method
that ends by calling itself runs as a loop, at any depth. CRuby keeps a frame
for every call. Here are two ways to count a String's bytes by recursion; only
the second ends in a tail call, since in the first the `+` runs after the call
returns.

``` {.ruby .cameo}
# Without a tail call: the + runs after the recursive call returns.
def len(s, i = 0)
  return 0 if s.getbyte(i).nil?

  1 + len(s, i + 1)
end

# With a tail call: the recursive call is the method's value.
def len_tc(s, i = 0)
  return i if s.getbyte(i).nil?

  len_tc(s, i + 1)
end

short = "x" * 1_000
p len(short), len_tc(short)

long = "x" * 1_000_000
p len_tc(long)
begin
  len(long)
rescue SystemStackError => e
  puts "len: #{e.class}: #{e.message}"
end
```

``` output
1000
1000
1000000
len: SystemStackError: stack level too deep
```

CRuby 3.4 raises `SystemStackError` for both methods on the long String, and
already at 100,000 bytes. Timed over 10 million calls in all, on a laptop on
2026-11-14, in milliseconds:

| Bytes × runs   |           | `len`          | `len_tc`       |
|----------------|-----------|----------------|----------------|
| 5,000 × 2000   | cameo     | 2793           | 1861           |
|                | CRuby 3.4 | 261            | 242            |
| 100,000 × 100  | cameo     | 2889           | 1858           |
|                | CRuby 3.4 | stack too deep | stack too deep |
| 1,000,000 × 10 | cameo     | stack too deep | 1856           |
|                | CRuby 3.4 | stack too deep | stack too deep |

-   **The tail call is a third faster** in cameo, at every size: a call that
    replaces its caller makes no frame and returns once, not once per level.
-   **Its depth has no limit.** `len_tc` takes the same time per call at a
    million levels as at five thousand.
-   **A call is still what costs.** Where CRuby runs the methods it is eight
    to ten times faster than cameo on them.
-   **Only a call in tail position gains.** In `fib(n - 1) + fib(n - 2)`
    neither call is one, and rewriting it as `fib2(fib2(acc, n - 1), n - 2)`
    makes the outer call a tail call and leaves as many calls: both take the
    same time.

A call that passes a block, a call under a `rescue` or an `ensure`, `super`
and `yield` keep their caller. A backtrace, a dump and the debugger do not
show the calls that were replaced; `cameo --keep-tail-frames` keeps the last
256 of a row for them, and makes a tail call about a tenth slower.

#### The IR: what a script compiles to {#ruby-ir}

cameo does not walk a script's syntax tree. It compiles the script to an
intermediate representation, the IR, and runs that. CRuby compiles to bytecode
too; what differs is that cameo's IR is a protobuf message, as every value is.
So the code of a script travels with its values: a [paused run](#ch-pause) is
written out with its code, a block goes to a worker or to another program as
its code and the values it captured, and a dump resumes in a process that
never saw the source. `cameo ir` prints the IR of a script without running it.
Take `len.cameo`, the two methods above in a class with one field:

``` {.ruby data-name="len.cameo"}
class Word
  string :text

  def len(i = 0)
    return 0 if text.getbyte(i).nil?

    1 + len(i + 1)
  end

  def len_tc(i = 0)
    return i if text.getbyte(i).nil?

    len_tc(i + 1)
  end
end

word = Word.new(text: "cameo")
puts word.len, word.len_tc
```

``` sh
cameo ir len.cameo
```

``` output
proc #0 <main>
  CLASS Word #1 3b0407905bbe03ac
  DROP
  GETCONST Word
  MAP 1
  CONST "cameo"
  SYM text
  MAP_SET
  SEND new 0 Word kw
  STORE word
  DROP
  LOAD word
  SEND len 0 word
  LOAD word
  SEND len_tc 0 word
  SEND puts 2 self

proc #1 <class:Word>
  FIELD (string :text)
  DROP
  DEF len #2
  DROP
  DEF len_tc #3

proc #2 len(i = …)
  default i:
    CONST 0
  SEND text 0 self vcall
  LOAD i
  SEND getbyte 1
  SEND nil? 0
  IF
    then:
      CONST 0
      RETURN
    else:
      CONST nil
  DROP
  CONST 1
  LOAD i
  CONST 1
  BINARY_PLUS
  SEND len 1 self
  BINARY_PLUS

proc #3 len_tc(i = …)
  default i:
    CONST 0
  SEND text 0 self vcall
  LOAD i
  SEND getbyte 1
  SEND nil? 0
  IF
    then:
      LOAD i
      RETURN
    else:
      CONST nil
  DROP
  LOAD i
  CONST 1
  BINARY_PLUS
  SEND len_tc 1 self tail
```

How to read it:

-   **A proc is one body**: the top level, a class body, a method, a block or
    a lambda. The script is four: the top level, the body of `Word` and its
    two methods.
-   **A class is defined by running its body.** `CLASS Word #1` runs proc #1,
    where `FIELD` declares the typed field and each `DEF` a method, by the
    number of its proc. The hexadecimal number after it is a digest of the
    body, by which cameo knows the same definition when it meets it again.
-   **An instruction works on a stack of values.** `LOAD` and `CONST` push
    one. `SEND len 1 self` calls `len` on self with the one value on top and
    pushes the result; a `SEND` without `self`, as `SEND getbyte 1`, takes its
    receiver from the stack too. `BINARY_PLUS` is `+`: on two Integers or two
    Floats the evaluator adds, on anything else it calls the method.
-   **Keyword arguments are a Hash.** `Word.new(text: "cameo")` builds it with
    `MAP` and `MAP_SET`, and `kw` on the `SEND` says the last value is the
    keywords.
-   **A field is read by a call.** `text` in a method is `SEND text 0 self`,
    the reader the field declaration defined; there is no instruction for an
    instance variable, since there are none.
-   **Control flow is nested, with no jumps.** An `IF` carries its two
    branches as lists, so a place in the code is a path of indices, which is
    what a dump stores as "where the script was".
-   **`DROP` ends a statement** by discarding its value. The instruction after
    it starts the next statement, which is where the debugger stops.
-   **`tail` marks a tail call.** The last `SEND` of `len_tc` has it and
    replaces its caller. In `len` the last instruction is the `BINARY_PLUS`,
    so its `SEND len` has none: this is the quickest way to see whether a call
    you meant to be a tail call is one.

The IR has a version, and a dump is resumed by a cameo with the same one: a
dump written by an older cameo is refused, not converted.
[doc/design/ir.md](#ch-design-ir) has every instruction and how the evaluator
runs them.

#### Time zones by name

An IANA zone name is accepted wherever Ruby takes a UTC offset:
`Time.new(…, in:)`, `Time.at(…, in:)`, `localtime`, `getlocal` and
`DateTime`'s offset. The time carries the zone, so its abbreviation, offset
and daylight saving follow the date. Ruby takes a zone object there; the
database is compiled in, so a name resolves on any machine.

``` {.ruby .cameo}
t = Time.at(1719835200, in: "Europe/Berlin")
p t, t.zone, t.utc_offset
p Time.new(2024, 1, 15, 12, 0, 0, in: "America/New_York").strftime("%H:%M %Z %:z")
p Time.utc(2024, 7, 1, 12).localtime("Asia/Tokyo")
```

``` output
2024-07-01 14:00:00 +0200
"CEST"
7200
"12:00 EST -05:00"
2024-07-01 21:00:00 +0900
```

#### `async`, `await` and promises {#ruby-async}

The [async guide](#ch-async) explains the model, shows it on one core, and
walks through every example.

Concurrency comes from the `Cameo` module, which a script includes: at the top
level for every method, or in a class body for that class. Without it `async`
and `await` are unknown; `Cameo.async` and `Cameo.await` work without it.

`async receiver, :method, args…` runs one method call on a goroutine of its
own, over copies of the receiver and the arguments, and returns the call's
*promise* at once: an Integer that names the result to come. `await` returns
the result, or raises the exception the call raised. It takes a promise, a
list of them, or `:all`, every call the current method started and has not
awaited. A call shares no memory with the script that started it: its
arguments are copied, a Proc among them with the values it captured, and a
class cannot be passed to one.

``` {.ruby .cameo}
include Cameo

class Worker
  int :factor

  def times(n)
    n * factor
  end

  def boom(msg)
    raise ArgumentError, msg
  end
end

w = Worker.new(factor: 3)

job = async w, :times, 4       # starts w.times(4) on a goroutine
p job                          # the promise: an Integer
p await job

jobs = [1, 2, 3].map { |n| async w, :times, n }
p await(jobs)                  # every result, in order

async w, :times, 10
async w, :times, 20
p await :all                   # every call this scope started and did not await

begin
  await w, :boom, "no"         # async and await in one step
rescue ArgumentError => e
  puts "the call raised: #{e.message}"
end
```

``` output
1
12
[3, 6, 9]
[30, 60]
the call raised: no
```

##### Every core at once

Go spreads the workers over the CPUs, so calls that compute run in parallel.
Each of these computes the 29th Fibonacci number the slow way:

``` {.ruby .cameo}
# Fibonacci on every core. async starts each call on a worker of its own, a
# goroutine that Go spreads over the CPUs, and returns its promise at once;
# await takes the list of promises and returns the results in order. The
# four calls run at once, one per core when there are four.
include Cameo

class Fib
  def of(n)
    n < 2 ? n : of(n - 1) + of(n - 2)
  end
end

fib = Fib.new
promises = [29, 29, 29, 29].map { |n| async fib, :of, n }
p promises
p await(promises)
```

``` output
[1, 2, 3, 4]
[514229, 514229, 514229, 514229]
```

On a machine with four cores the four calls take 0.8 seconds, where running
them one after another takes 1.4; `time` shows the cores at work, with more
CPU time than wall time:

``` sh
$ time cameo examples/fibonacci.cameo
[1, 2, 3, 4]
[514229, 514229, 514229, 514229]

real    0m0.762s
user    0m2.381s
sys 0m0.311s
```

##### Settling a promise by hand

`promise` makes a pending promise, which `resolve promise, value` settles with
a value and `fail promise, exception` with an exception, from the script or
from a worker it was passed to: a promise is an Integer, so it travels like
any other value. `fail` takes what `raise` takes after the promise; with
anything but a promise first it raises, as Ruby's `fail` does. A promise is
settled once. A Go host settles promises the same way when it does I/O for a
script; see [promises in the host](#go-promises).

``` {.ruby .cameo}
include Cameo

class Courier
  def deliver(promise, parcel)
    resolve(promise, "#{parcel}, delivered")
  end

  def lose(promise, parcel)
    fail(promise, KeyError, "#{parcel} is lost")
  end
end

courier = Courier.new
parcel = promise               # a pending promise, nobody's result yet
async courier, :deliver, parcel, "books"
p await(parcel)

missing = promise
async courier, :lose, missing, "a lamp"
begin
  await missing
rescue KeyError => e
  puts e.message
end

begin
  resolve parcel, "again"
rescue ArgumentError => e
  puts e.message
end
```

``` output
"books, delivered"
a lamp is lost
promise 1 is settled already
```

#### Protobuf {#ruby-proto}

Every class is a protobuf message. A field's number is derived from a hash of
its name, so it stays the same when fields are added, removed or reordered;
`number:` gives one from 1 to 4096 instead, as in `int :x, number: 1`, which
is shorter on the wire. `reserved` keeps a removed field's number and name out
of use. `to_proto` returns the wire form, `from_proto` reads it back, and
`Point.proto` returns the `.proto` text. `cameo proto script.cameo` writes the
`.proto` files of every class a script defines.

``` {.ruby .cameo}
class Point
  int :x, :y
end

pt = Point.new(x: 3, y: 4)
data = pt.to_proto
p data.bytesize
p Point.from_proto(data) == pt
puts Point.proto
```

``` output
8
true
message Point {
  optional int64 x = 64843;
  optional int64 y = 64608;
}
```

A Symbol becomes a String where a value leaves cameo, in a protobuf message or
an `async` call, unless an `enum` field declares it.
