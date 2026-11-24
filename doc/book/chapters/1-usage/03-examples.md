## cameo by example {#ch-examples}

::: source
from `doc/gh_pages/examples.html`
:::

Seven short programs, each with what it prints. If you know Ruby you can read
all of them; the comments say where cameo goes its own way.

1.  [Methods and blocks](#examples-hello)
2.  [A class with typed fields](#examples-classes)
3.  [Nothing changes in place](#examples-immutable)
4.  [Collections](#examples-collections)
5.  [Pattern matching](#examples-patterns)
6.  [Exceptions](#examples-exceptions)
7.  [Async](#examples-async)

To run one, save it as `hello.cameo` and say `cameo hello.cameo`, or type it
into `cameo irb`. Building the binary needs Go alone:

``` sh
git clone https://github.com/cameo-lang/cameo
cd cameo
make install        # puts cameo into GOBIN
```

### Methods and blocks {#examples-hello}

Methods, keyword arguments, string interpolation, ranges and blocks are
Ruby's.

``` {.ruby .cameo}
def greet(name, greeting: "Hello")
  "#{greeting}, #{name}!"
end

puts greet("world")
puts greet("cameo", greeting: "Welcome")

squares = (1..5).map { |n| n * n }
p squares, squares.sum, squares.select(&:even?)
```

``` output
Hello, world!
Welcome, cameo!
[1, 4, 9, 16, 25]
55
[4, 16]
```

### A class with typed fields {#examples-classes}

A class declares its fields with their types, where Ruby has instance
variables. `new` takes them by name, `==` compares them, and `with` returns a
copy with some of them changed.

``` {.ruby .cameo}
class Account
  string :owner
  int :cents

  def deposit(amount)
    with(cents: cents + amount)
  end

  def to_s
    format("%s: %.2f", owner, cents / 100.0)
  end
end

a = Account.new(owner: "Ada", cents: 1_000)
b = a.deposit(250)

puts a, b
p a == Account.new(owner: "Ada", cents: 1_000)
```

``` output
Ada: 10.00
Ada: 12.50
true
```

A field's type is `int`, `float`, `string`, `bool`, `bytes`, one of protobuf's
sized numbers, a `list`, `set` or `map`, or another class with `ref`. [Ruby vs
cameo](#ruby-fields) has them all.

### Nothing changes in place {#examples-immutable}

Every value is immutable. An assignment makes a changed copy and rebinds the
variable it names; whoever else held the old value still holds it. A call of
one of Ruby's mutators, such as `<<` or `push`, raises, and the message names
the form to write.

``` {.ruby .cameo}
list = [3, 1, 2]
same = list
list += [4]               # a new list; same still holds the old one
list = list.sort

p list, same

prices = { tea: 3, cake: 5 }
prices[:tea] = 4          # rebinds prices to a changed copy
p prices

begin
  list << 5               # Ruby's mutators raise: nothing changes in place
rescue CameoError => e
  puts e.message
end
```

``` output
[1, 2, 3, 4]
[3, 1, 2]
{tea: 4, cake: 5}
cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (see https://15b.it/mut)
```

### Collections {#examples-collections}

Arrays, hashes, sets and ranges have Ruby's Enumerable. A block captures the
values around it and cannot assign to a variable outside itself, so a sum is a
`reduce` or a `sum`, and a hash is built with `each_with_object`, `to_h` or
`group_by`.

``` {.ruby .cameo}
words = %w[apple banana cherry avocado blueberry]

by_letter = words.group_by { |w| w[0] }
p by_letter

total = words.reduce(0) { |sum, w| sum + w.length }
p total

counts = words.each_with_object({}) { |w, seen| seen[w[0]] = (seen[w[0]] || 0) + 1 }
p counts

longest = words.max_by(&:length)
puts "longest: #{longest}"
```

``` output
{"a" => ["apple", "avocado"], "b" => ["banana", "blueberry"], "c" => ["cherry"]}
33
{"a" => 2, "b" => 2, "c" => 1}
longest: blueberry
```

### Pattern matching {#examples-patterns}

`case`/`in` takes an instance apart by its fields, and arrays and hashes as in
Ruby.

``` {.ruby .cameo}
class Circle
  float :radius
end

class Rect
  float :width, :height
end

def area(shape)
  case shape
  in Circle(radius:) then 3.14159 * radius * radius
  in Rect(width:, height:) then width * height
  in [first, *rest] then area(first) + rest.sum { |s| area(s) }
  end
end

p area(Circle.new(radius: 1.0))
p area([Rect.new(width: 2.0, height: 3.0), Rect.new(width: 1.0, height: 1.0)])
```

``` output
3.14159
7.0
```

### Exceptions {#examples-exceptions}

`raise`, `rescue`, `ensure` and `retry` are Ruby's. An exception class may
declare fields, which `new` takes after the message.

``` {.ruby .cameo}
class OutOfStock < StandardError
  string :item
end

def take(stock, item)
  raise OutOfStock.new("no #{item} left", item: item) if stock.fetch(item, 0) == 0
  stock.merge(item => stock[item] - 1)
end

stock = { "tea" => 1 }
stock = take(stock, "tea")
begin
  take(stock, "tea")
rescue OutOfStock => e
  puts "#{e.message} (#{e.item})"
ensure
  p stock
end
```

``` output
no tea left (tea)
{"tea" => 0}
```

### Async {#examples-async}

`async` starts a method call on a goroutine of its own and returns a promise;
`await` waits for the result. Values are immutable, so the calls share nothing
and need no locks.

``` {.ruby .cameo}
include Cameo

class Page
  string :name

  def words
    name.length * 100      # stands in for real work
  end
end

pages = %w[home about pricing].map { |n| Page.new(name: n) }
jobs = pages.map { |page| async page, :words }   # each on its own goroutine
p jobs.map { |job| await job }
```

``` output
[400, 500, 700]
```

The [async page](#ch-async) has the model and eight more examples. The scripts
under [examples/](https://github.com/cameo-lang/cameo/tree/main/examples) in
the repository, 74 of them, are cameo's specification: each has the output it
must print beside it, and the tests run them all.
