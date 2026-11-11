package eval

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/syntax"
)

// prints runs src and holds what it prints to want; an error that ends
// the script is the last line. The expected outputs are what the tree
// walker printed for these scripts before the evaluator replaced it.
func prints(t *testing.T, src, want string) {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	var out bytes.Buffer
	in := New(&out, &out)
	if err := in.Run("t.cameo", prog); err != nil {
		out.WriteString(in.FormatError(err) + "\n")
	}
	assert.Equals(t, out.String(), want)
}

func TestStatements(t *testing.T) {
	prints(t, `a = 3
b = a * 2 + 1
puts b
puts "a is #{a}, b is #{b}"
puts [a, *[b, 9], :sym].inspect
puts({x: a, **{y: b}}.inspect)
puts (1..a).to_a.inspect, (1...a).to_a.inspect
puts a > 2 && b > 2, a > 5 || nil.inspect, !a
`, "7\na is 3, b is 7\n[3, 7, 9, :sym]\n{x: 3, y: 7}\n[1, 2, 3]\n[1, 2]\ntrue\nnil\nfalse\n")
}

func TestAMethodDefinition(t *testing.T) {
	prints(t, `def fib(n)
  if n < 2
    n
  else
    fib(n - 1) + fib(n - 2)
  end
end
puts fib(15)
`, "610\n")
}

func TestDefinitions(t *testing.T) {
	prints(t, `module Shape
  SIDES = 0
  def describe = "#{self.class.name} with #{sides} sides"
  def self.unit = 1
end
module Named
  def label = "named #{name}"
end
class Polygon
  include Shape
  extend Named
  attribute :registry, default: []
  string :name
  int :sides, default: 3
  list :tags, of: :string
  reserved 7, :legacy
  self.registry = registry + ["polygon"]
  def initialize(name, sides = 3)
    self.name = name
    self.sides = sides
  end
  def self.name = "Polygon"
  def area = raise(NotImplementedError, "no area")
  alias to_s describe
end
class Square < Polygon
  ref :twin, Polygon
  enum :kind, values: [:small, :large]
  def initialize(name) = super(name, 4)
  def area = sides * sides
  def self.name = "Square"
end
class Shape::Circle
  float :r
  def initialize(r) = self.r = r
end
module Shape::Util
  def self.double(x) = x * 2
end
s = Square.new("sq")
puts s.describe, s.to_s, s.area, Square.label, Polygon.registry.inspect
puts Square.ancestors.first(3).inspect, Shape.unit, Shape::SIDES
puts Shape::Circle.new(1.5).r, Shape::Util.double(4)
puts Square.fields.map { |f| f.to_s }.sort.inspect
puts s.with(twin: Polygon.new("p"), kind: :large).inspect
class Polygon
  def corners = sides
end
puts Polygon.new("tri").corners, s.respond_to?(:corners)
def top(x) = x + 1
alias up top
puts up(1)
klass = Class.new(Polygon) do
  def area = 42
end
puts klass.new("anon").area
puts (def helper; end).inspect
`, `Square with 4 sides
Square with 4 sides
16
named Square
["polygon"]
[Square, Polygon, Shape]
1
0
1.5
8
["kind", "name", "sides", "tags", "twin"]
Square(name: "sq", sides: 4, tags: [], twin: Polygon(name: "p", sides: 3, tags: []), kind: :large)
3
false
2
42
:helper
`)
	prints(t, "class A < 5\nend\n", "t.cameo:1: superclass must be a Class (an instance of Integer given) (TypeError)\n")
	prints(t, "class A\n  int :x\n  int :x\nend\n", "t.cameo:3: field x is already declared in A (CameoError)\n")
	prints(t, "class A\n  include Integer\nend\n", "t.cameo:2: wrong argument type class Integer (expected Module) (TypeError)\n")
	prints(t, "s = \"a\\xffb\"\nputs s.bytes.inspect, s.encoding, s.valid_encoding?\n", "[97, 255, 98]\nUTF-8\nfalse\n")
	prints(t, "class A\n  def boom = raise(\"in body\")\n  boom2\nend\nputs defined?(A).inspect\n", "t.cameo:3: undefined local variable or method 'boom2' for class A (NameError)\n")
}

func TestLoopsAndJumps(t *testing.T) {
	prints(t, `i = 0
total = 0
while i < 10
  i += 1
  next if i == 3
  break if i == 8
  total += i
end
puts total
puts (while false do end).inspect
`, "25\nnil\n")
}

func TestBlocks(t *testing.T) {
	prints(t, `puts [1, 2, 3].map { |x| x * 2 }.inspect
double = ->(x) { x * 2 }
puts double.(4), double.lambda?, double.arity
add = proc { |a, b = 10, *rest| a + b + rest.sum }
puts add.call(1), add.call(1, 2, 3), add.arity
fact = ->(n) { n <= 1 ? 1 : n * fact.(n - 1) }
puts fact.(5)
puts [3, 1, 2].sort_by { |x| -x }.first
puts [[1, 2], [3, 4]].map { |a, b| a + b }.inspect
puts [1, 2, 3].each_with_index.map { |x, i| x * i }.inspect
puts (1..3).reduce(0) { |sum, x| sum + x }
puts [1, 2, 3, 4].find { |x| x > 2 }
`, "[2, 4, 6]\n8\ntrue\n1\n11\n6\n-2\n120\n3\n[3, 7]\n[0, 2, 6]\n6\n3\n")
}

func TestCalls(t *testing.T) {
	prints(t, `def greet(name, greeting = "hello", *rest, punct: "!", **opts, &blk)
  s = "#{greeting} #{name}#{punct}"
  s = blk.call(s) if blk
  return s, rest, opts
end
def each_twice
  return :none unless block_given?
  yield 1
  yield 2, 3
end
def pass(*args, **kw) = greet(*args, **kw)
puts greet("ann").inspect
puts greet("bob", "hi", 1, 2, punct: "?", x: 1) { |s| s.upcase }.inspect
puts pass("cy", punct: ".").inspect
up = :upcase
puts ["a", "b"].map(&up).inspect
puts each_twice.inspect
each_twice { |a, b| puts [a, b].inspect }
x = nil
puts x&.length.inspect, "abc"&.length
puts 7 / 2, 7 % 3, 2**10, 1.5 + 2, 10.fdiv(4)
puts [1, 2].frozen?
`, `["hello ann!", [], {}]
["HI BOB?", [1, 2], {x: 1}]
["hello cy.", [], {}]
["A", "B"]
:none
[1, nil]
[2, 3]
nil
3
3
1
1024
3.5
2.5
true
`)
}

func TestRescue(t *testing.T) {
	prints(t, `tries = 0
r = begin
  tries += 1
  raise ArgumentError, "no" if tries < 3
  "ok after #{tries}"
rescue TypeError, ArgumentError => e
  retry
else
  :unused
ensure
  puts "ensure ran"
end
puts r.inspect
x = begin
  Integer("zz")
rescue => e
  "rescued #{e.class}: #{e.message}"
end
puts x
y = begin
  1 + [nil.foo]
rescue NoMethodError
  begin
    raise "inner"
  rescue => inner
    "#{inner.message} caused by #{inner.cause.class}"
  end
end
puts y
z = begin
  10
ensure
  20
end
puts z
errors = [ZeroDivisionError, KeyError]
w = begin
  1 / 0
rescue *errors => e
  e.class
end
puts w
`, `ensure ran
:unused
rescued ArgumentError: invalid value for Integer(): "zz"
inner caused by NoMethodError
10
ZeroDivisionError
`)
	prints(t, "begin\n  raise \"boom\"\nensure\n  puts \"cleanup\"\nend\n", "cleanup\nt.cameo:2: boom (RuntimeError)\n")
	prints(t, "begin\n  raise \"a\"\nrescue TypeError\n  1\nend\n", "t.cameo:2: a (RuntimeError)\n")
	prints(t, "begin\n  1\nrescue 5\n  2\nend\nbegin\n  raise \"x\"\nrescue 5\n  2\nend\n", "t.cameo: class or module required for rescue clause (TypeError)\n")
}

func TestCase(t *testing.T) {
	prints(t, `kinds = [1, "s", :sym, 2.5, nil, [1, 2], 40].map do |v|
  case v
  when Integer, Float then v > 10 ? "big" : "number"
  when "s", /x/ then "string"
  when *[:sym, :other] then "symbol"
  when nil then "nil"
  else "other"
  end
end
puts kinds.inspect
n = 5
puts(case
     when n < 3 then "low"
     when n < 9 then "mid"
     end)
puts (case n when 1 then :one end).inspect
`, "[\"number\", \"string\", \"symbol\", \"number\", \"nil\", \"other\", \"big\"]\nmid\nnil\n")
}

func TestPatterns(t *testing.T) {
	prints(t, `def kind(v)
  case v
  in Integer | Float => n if n > 100 then "big #{n}"
  in Integer | Float then "number"
  in [] then "empty"
  in [first, *rest] unless rest.empty? then "list of #{first} and #{rest.size}"
  in [only] then "one #{only}"
  in {name: String => name, **rest} then "named #{name} with #{rest.inspect}"
  in {x:, y:} then "point #{x},#{y}"
  in [*, :needle, *post] then "found before #{post.inspect}"
  in String then "string"
  else "other"
  end
end
puts kind(500), kind(5), kind([]), kind([1, 2, 3]), kind([7]), kind({name: "ann", age: 3})
puts kind({x: 1, y: 2}), kind("s"), kind(nil)
config = {db: {user: "admin", port: 5432}}
config => {db: {user:, port: Integer => port}}
puts user, port
if 5 in Integer
  puts true
end
unless "s" in Integer
  puts false
end
case [1, [2, 3]]
in [a, [b, *c]]
  puts a + b + c.sum
end
case {status: "ok"}
in {status: "ok" | "fine", **nil}
  puts "exact"
end
`, `big 500
number
empty
list of 1 and 2
one 7
named ann with {age: 3}
point 1,2
string
other
admin
5432
true
false
6
exact
`)
	prints(t, "case 5\nin String then 1\nend\n", "t.cameo:1: 5 (NoMatchingPatternError)\n")
	prints(t, "x = {a: 1}\nx => {b:}\n", "t.cameo:2: {a: 1} (NoMatchingPatternError)\n")
}

func TestAssignmentForms(t *testing.T) {
	prints(t, `a, b = 1, 2
c, *d = [3, 4, 5]
*e, f = 6, 7, 8
g, h = [9]
puts [a, b, c, d, e, f, g, h].inspect
x = (i, j = 1, 2)
puts x.inspect
LIMIT = 10
puts LIMIT, ::LIMIT, Math::PI.round(2), Comparable.class
word = "b"
puts "abc" =~ /#{word}c/, "abc".match?(/B/i), /a#{/b/}c/.source
puts defined?(a).inspect, defined?(zzz).inspect, defined?(LIMIT), defined?(Nope).inspect
puts defined?(puts), defined?(a.nope).inspect, defined?(self), defined?(q = 1), defined?(42)
puts defined?(String.new), defined?(Nope::Deeper).inspect
def blocky = defined?(yield).inspect
puts blocky, blocky { 1 }
`, `[1, 2, 3, [4, 5], [6, 7], 8, 9, nil]
[1, 2]
10
10
3.14
Module
1
true
a(?-mix:b)c
"local-variable"
nil
constant
nil
method
nil
self
assignment
expression
method
nil
nil
"yield"
`)
	prints(t, "Nope::X\n", "t.cameo:1: uninitialized constant Nope (NameError)\n")
	prints(t, "x = 5\nx::Y\n", "t.cameo:2: an instance of Integer is not a class/module (TypeError)\n")
	prints(t, "String = 1\n", "t.cameo:1: cannot reassign the builtin constant String (CameoError)\n")
}

func TestSuperWithAndAsync(t *testing.T) {
	prints(t, `class Animal
  string :name
  def initialize(name) = self.name = name
  def speak(loud = false, *rest, punct: ".", &blk)
    s = "#{name} speaks#{punct}"
    s = s.upcase if loud
    blk ? blk.call(s) : s
  end
  def describe(prefix, punct: ".") = "#{prefix} #{name}#{punct}" + (block_given? ? yield : "")
  def self.kinds = ["animal"]
end
class Dog < Animal
  def speak(loud = false, *rest, punct: "!", &blk) = "dog: " + super
  def twice(&blk) = super() rescue "no twice above"
  def describe(prefix) = super("good", punct: "?") { " (block)" }
  def self.kinds = super + ["dog"]
end
d = Dog.new("rex")
puts d.speak, d.speak(true), d.speak { |s| "<#{s}>" }
puts d.describe("x"), d.twice, Dog.kinds.inspect
e = d.with(name: "max")
puts e.name, d.name
h = {a: 1}
h[:b] = 2
l = [1, 2]
l[0] = 9
e.name = "bo"
puts h.inspect, l.inspect, e.name
include Cameo
class Work
  def sq(x, by: 1) = x * x * by
  def boom = raise(ArgumentError, "boom")
end
w = Work.new
a = async w, :sq, 3
b = async w, :sq, 4, by: 2
puts await(a), await([a, b]).inspect
puts await(w, :sq, 5)
c = Cameo.async w, :sq, 6
puts Cameo.await(c), (await :all).inspect
begin
  await async(w, :boom)
rescue ArgumentError => err
  puts "failed: #{err.message}"
end
`, `dog: rex speaks.
dog: REX SPEAKS.
dog: <rex speaks.>
good rex? (block)
no twice above
["animal", "dog"]
max
rex
{a: 1, b: 2}
[9, 2]
bo
9
[9, 32]
25
36
[]
failed: boom
`)
	prints(t, "def f = super\nf\n", "t.cameo:1: super: no superclass method 'f' for main (NoMethodError)\n")
	prints(t, "class W\n  def go = 1\nend\nx = async W.new, :go\n", "t.cameo:4: async comes from the Cameo module: include Cameo, or call Cameo.async (NoMethodError)\n")
	prints(t, "include Cameo\nx = async 1, 2\n", "t.cameo:2: async names the method by a symbol, not an instance of Integer (TypeError)\n")
}

func TestErrorsNameTheLineOfTheirInstruction(t *testing.T) {
	prints(t, `def f(x)
  x + nope
end
f(1)
`, "t.cameo:2: undefined local variable or method 'nope' for main (NameError)\n")
	prints(t, "x = 1\ny = x.missing(2)\n", "t.cameo:2: undefined method 'missing' for an instance of Integer (NoMethodError)\n")
	prints(t, "def f = yield\nf\n", "t.cameo:1: no block given (yield) (LocalJumpError)\n")
	prints(t, "a = [1]\na << 2\n", "t.cameo:2: cameo does not support mutating calls without rebinding; use a = a << 1, +, or append (CameoError)\n")
}
