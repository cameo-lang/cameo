# Writing cameo programs: a guide for agents

This file is for a model that writes cameo scripts. It says what to
write, what to leave out, and how to check the result. The guidance for
agents that work on cameo itself is in [CLAUDE.md](CLAUDE.md).

Every Ruby snippet below is run by `go test ./tests`. The full documentation is on the site,
[cameo-lang.github.io/cameo](https://cameo-lang.github.io/cameo/):
[examples](https://cameo-lang.github.io/cameo/examples.html),
[Ruby vs cameo](https://cameo-lang.github.io/cameo/ruby.html),
[async](https://cameo-lang.github.io/cameo/async.html) and
[I/O](https://cameo-lang.github.io/cameo/io.html). The complete list of
differences is [doc/cameo-vs-ruby.md](doc/cameo-vs-ruby.md), and the
libraries a script may `require` are in [doc/stdlib.md](doc/stdlib.md).

## The rule

**Write Ruby, and leave out everything that changes a value in place.**
cameo has Ruby's syntax and most of its core classes. Every value is
immutable, so a program is written as values computed from values.

Then check it before you hand it over:

```sh
cameo check script.cameo     # every error, with its line, without running
cameo script.cameo           # run it
```

`cameo check` names the construct it refuses and what to write instead.
Fix what it says and check again; do not guess around it.

## Leave these out

| Do not write | Write instead |
|---|---|
| `@x`, `attr_accessor`, `attr_reader`, `def x=(v)` | a typed field: `int :x` in the class body; read it as `x` |
| `a << 1`, `a.push(1)`, `h.delete(k)`, `s << "x"` as statements | an assignment: `a += [1]`, `a = a << 1`, `h = h.except(k)`, `s += "x"` |
| `a.sort!`, `s.gsub!(…)`, `h.merge!(o)`: every bang method of Ruby's core | the copying form, assigned: `a = a.sort` |
| `list.each { \|x\| total += x }`: a block that assigns to a variable outside it | `sum`, `reduce`, `map`, `count`, `each_with_object`, or a `while` loop |
| `@@x`, `$x` | a constant, a field or a local |
| `Struct`, `Data`, `OpenStruct` | a class with typed fields |
| `private`, `protected`, `module_function` | nothing: every method a script defines is public |
| `define_method`, `eval`, `instance_variable_get`, `class << self`, `def obj.x` | a plain `def`; `def self.x` for a class method |
| `Thread`, `Mutex`, `Fiber`, `sleep`, `system`, backticks, `fork` | `async` and `await` |
| `File.read`, `File.write` unless you were told I/O is allowed; `Dir`, `ENV`, `gets`, `STDIN`, sockets | the functions the host gives the script; ask what they are |
| `require` of a library not in [doc/stdlib.md](doc/stdlib.md), `require_relative`, `load` | one file per script; `json`, `date`, `time`, `set`, `csv`, `uri`, `stringio`, `strscan`, `bigdecimal`, `digest`, `base64`, `securerandom` and others are built in |
| `Rational`, `Complex`, `1r`, `2i`, `ObjectSpace`, `GC` | none |
| `equal?`, `object_id` | `==`, which compares contents |
| `"\xFF"`, a literal with bytes that are not UTF-8 | a binary literal, `b"\xFF"` |
| `"v=#{bytes}"`, `format("%s", bytes)` with a binary String | `bytes.force_encoding("UTF-8")` first when the bytes are text; `+` to join bytes |
| `s.getbyte(i)` on text with characters beyond ASCII, `s.setbyte` on text | `s.b.getbyte(i)`; a binary String for `setbyte` |

## Classes: typed fields and `with`

A class declares its fields with their types. `new` takes them by name
when the class has no `initialize`. A method that would change the
instance returns a changed copy with `with`, and the caller assigns it.

```ruby
class Order
  string :customer
  list :items, of: :string
  int :total_cents, default: 0

  def add(item, cents) = with(items: items + [item], total_cents: total_cents + cents)
  def to_s = "#{customer}: #{items.join(", ")} (#{format("%.2f", total_cents / 100.0)})"
end

order = Order.new(customer: "Ada", items: [])
order = order.add("tea", 350).add("cake", 500)
puts order                 # Ada: tea, cake (8.50)
```

The field types are `int`, `float`, `string`, `bool`, `bytes`, protobuf's
sized numbers (`int32`, `uint64`, …), `list` and `set` with `of:`, `map`,
`ref :name, OtherClass` and `enum`. A wrong type raises `TypeError`.
Inside `initialize`, set a field with `self.x = v`.

## Collections without accumulators

A block captures the values around it and cannot assign to a variable
outside itself. Reach for the method that returns the result.

```ruby
words = %w[tea cake tea scone tea]

counts = words.tally                                   # {"tea" => 3, "cake" => 1, "scone" => 1}
total = words.sum(&:length)                            # 18
longest = words.reduce { |best, w| w.length > best.length ? w : best }
index = words.each_with_object({}) { |w, seen| seen[w] = true }.keys

i = 0                      # a counter lives in a while loop
found = nil
while i < words.size && found.nil?
  found = i if words[i] == "scone"
  i += 1
end
```

`h[k] = v` and `a.x = 1` are assignments: they rebind `h` and `a` to a
changed copy. Any other variable that held the old value keeps it.

## async and await

Use them where pieces of work do not depend on each other: one call per
item of a list, several requests to the host, a slow computation beside
other work. Do not use them for a single call or for steps that feed each
other; plain calls are simpler and as fast.

```ruby
include Cameo                       # once, at the top level

class Report
  string :name

  def score = (1..200_000).sum { |n| n % 7 } + name.length
end

reports = %w[north south east west].map { |n| Report.new(name: n) }

jobs = reports.map { |r| async r, :score }     # start every call first
scores = jobs.map { |job| await job }          # then collect
p reports.map(&:name).zip(scores).to_h

class Flaky
  def run = raise(ArgumentError, "bad input")
end

begin
  await async(Flaky.new, :run)                 # a worker's exception is
rescue ArgumentError => e                      # raised where it is awaited
  puts "failed: #{e.message}"
end
```

- `async receiver, :method, *args` starts the call on a goroutine and
  returns a promise. Values are immutable, so the call shares nothing
  with the caller and needs no lock.
- `await promise` returns the result or raises the call's exception.
  `await [a, b]` awaits several, and `await :all` everything the current
  scope started.
- Start all the calls before the first `await`. `await async(…)` in a
  loop runs them one after another.
- A function of the host that does I/O usually returns a promise:
  `page = await Web.fetch(url)`.

## Text and bytes

A String is text (UTF-8) or bytes (binary), and cameo keeps the two
apart where Ruby lets them mix. Text holds whole characters alone: no
String is "invalid UTF-8".

```ruby
text = "héllo"                         # UTF-8
bytes = b"\xFF\x00"                    # binary; "…".b makes one of text
p text.length, bytes.bytesize          # 5 and 2

packet = b"ID:" + text                 # binary: bytes joined with anything are bytes
p packet.encoding                      # #<Encoding:BINARY (ASCII-8BIT)>

again = packet[3..].force_encoding("UTF-8")   # say that bytes are text
puts "got #{again}"                    # interpolation takes text alone
```

- Joining with `+`, `<<` or `join` gives bytes as soon as one part is
  binary.
- An interpolation and `format` raise `Encoding::CompatibilityError`
  for a binary String, and so do `upcase`, `strip`, `split` and the
  other methods that read characters. `force_encoding("UTF-8")` says
  the bytes are text, and raises when they are not.
- What reads text raises for other bytes, and never returns a broken
  String: `File.read` (use `mode: "rb"` for bytes), `JSON.parse`,
  `CGI.unescape`, `URI.decode_www_form_component`.

## Recursion: end in the call

A call that is the value of its method replaces its caller, so a method
that ends by calling itself runs as a loop and is not limited by the
stack. Ruby keeps a frame per call; cameo does not. Carry the result in
an argument instead of computing after the call returns.

```ruby
def sum_to(n, acc = 0)
  return acc if n == 0

  sum_to(n - 1, acc + n)          # a tail call: any depth
end

def sum_slow(n) = n == 0 ? 0 : n + sum_slow(n - 1)   # the + runs after: SystemStackError when deep

puts sum_to(1_000_000)
```

`cameo ir script.cameo` prints what the script compiles to; a tail call
is a `SEND` marked `tail`.

## Before you finish

1. `cameo check` reports nothing.
2. The script runs and prints what was asked for.
3. No value is expected to change behind a variable: every change is an
   assignment.
4. Independent work is started with `async` and collected with `await`;
   everything else is plain calls.
