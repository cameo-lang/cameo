package eval

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// run evaluates a script and returns its standard output and the error that
// ended it.
func run(t *testing.T, src string) (string, error) {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out bytes.Buffer
	in := New(&out, &out)
	err = in.Run("t.cameo", prog)
	if err != nil {
		err = &formatted{in.FormatError(err), err}
	}
	return out.String(), err
}

type formatted struct {
	msg string
	err error
}

func (f *formatted) Error() string { return f.msg }

func expectOutput(t *testing.T, src, want string) {
	t.Helper()
	got, err := run(t, src)
	if err != nil {
		t.Fatalf("%s\nfailed: %v\noutput: %s", src, err, got)
	}
	if got != want {
		t.Fatalf("%s\ngot:\n%swant:\n%s", src, got, want)
	}
}

func expectError(t *testing.T, src, want string) {
	t.Helper()
	got, err := run(t, src)
	if err == nil {
		t.Fatalf("%s\nran without error; output: %s", src, got)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s\nerror is %q, want it to contain %q", src, err.Error(), want)
	}
}

func TestErrorsNameLineAndClass(t *testing.T) {
	expectError(t, "x = 1\nx.nope\n", "t.cameo:2: undefined method 'nope' for an instance of Integer (NoMethodError)")
	expectError(t, "nil.foo\n", "undefined method 'foo' for nil (NoMethodError)")
	expectError(t, "def f(a); end\nf\n", "t.cameo:2: wrong number of arguments (given 0, expected 1) (ArgumentError)")
	expectError(t, "def f(a, b = 1); end\nf(1, 2, 3)\n", "(given 3, expected 1..2)")
	expectError(t, "Nope\n", "uninitialized constant Nope (NameError)")
	expectError(t, "1 / 0\n", "divided by 0 (ZeroDivisionError)")
	expectError(t, "raise \"custom\"\n", "t.cameo:1: custom (RuntimeError)")
	expectError(t, "1 < \"a\"\n", "comparison of Integer with String failed (ArgumentError)")
}

func TestScopes(t *testing.T) {
	expectOutput(t, "x = 1\n[1].each { |i| x = i + 10; y = 5 }\nputs x\nputs defined?(y).inspect rescue puts \"nil\"\n", "11\nnil\n")
	expectOutput(t, "def m\n  v = 1\n  l = -> { v += 1 }\n  l.call\n  l.call\n  v\nend\nputs m\n", "3\n")
	expectOutput(t, "x = 10\ndef m = x\nbegin\n  m\nrescue NameError, NoMethodError => e\n  puts e.class\nend\n", "NoMethodError\n")
}

func TestBlocksAndReturns(t *testing.T) {
	expectOutput(t, "def m\n  [1, 2, 3].each { |x| return x if x == 2 }\n  :none\nend\nputs m\n", "2\n")
	expectOutput(t, "r = [1, 2, 3].each { |x| break x * 2 if x == 2 }\nputs r\n", "4\n")
	expectOutput(t, "l = -> { return 5; 6 }\nputs l.call\n", "5\n")
	expectOutput(t, "def m\n  yield 1, 2\nend\nm { |a, b| puts a + b }\nm { |a| puts a }\n", "3\n1\n")
	expectOutput(t, "def m\n  yield [1, 2]\nend\nm { |a, b| puts b }\n", "2\n")
	expectOutput(t, "def m(&b) = b\nputs m.inspect\nputs m { 1 }.call\n", "nil\n1\n")
}

func TestClassesAreValuesByDigest(t *testing.T) {
	src := `
class A
  def hi = "a"
end
first = A.new
class A
  def hi = "a"
end
puts first.class == A
class A
  def hi = "A"
end
puts first.hi
puts A.new.hi
puts first.class == A
puts first.class.name
`
	expectOutput(t, src, "true\na\nA\nfalse\nA\n")
	expectOutput(t, "class A\n def x = 1\n def x = 2\nend\np A.new.x\n", "2\n")
	expectError(t, "class Integer\n def x; end\nend\n", "cannot redefine the builtin class Integer")
}

func TestExceptions(t *testing.T) {
	expectOutput(t, "begin\n  raise ArgumentError\nrescue => e\n  puts e.message\n  puts e.class\nend\n", "ArgumentError\nArgumentError\n")
	expectOutput(t, "class E < StandardError; end\nbegin\n  raise E, \"m\"\nrescue RuntimeError\n  puts 1\nrescue E => e\n  puts e.message\nensure\n  puts :done\nend\n", "m\ndone\n")
	expectOutput(t, "def m\n  yield\nrescue ZeroDivisionError\n  :rescued\nelse\n  :fine\nend\nputs m { 1 / 0 }\nputs m { 1 }\n", "rescued\nfine\n")
	expectOutput(t, "x = begin\n  Integer(\"zz\")\nrescue ArgumentError\n  -1\nend\nputs x\n", "-1\n")
	if _, err := run(t, "raise Exception, \"top\"\n"); err == nil {
		t.Fatal("Exception is not rescued by a bare rescue, and ends the run")
	}
	_ = object.Nil
}

func TestDefinedNamesWithoutEvaluating(t *testing.T) {
	// defined? answers in Ruby's words and does not call what it names.
	src := `
class A; end unless defined?(A)
x = 1
p defined?(A), defined?(Zz), defined?(x), defined?(zz), defined?(puts), defined?(x.succ), defined?(x.nope), defined?(self)
p Object.new, Object.new.class, 42.infinite?
p (0..3).reverse_each { }, (0..9).to_a.values_at(1..3, 8..12, -2..-1, 0)
`
	expectOutput(t, src, "\"constant\"\nnil\n\"local-variable\"\nnil\n\"method\"\n\"method\"\nnil\n\"self\"\n#<Object>\nObject\nnil\n0..3\n[1, 2, 3, 8, 9, nil, nil, nil, 8, 9, 0]\n")
}

func TestClassNewTakesFields(t *testing.T) {
	// Class.new's block is a class body: fields and defs belong to it.
	src := `
Pt = Class.new do
  int :x
  def double = x * 2
end
p Pt.new(x: 4).double, Pt.new(x: 1), Class.new
`
	expectOutput(t, src, "8\nPt(x: 1)\n#<Class>\n")
}

func TestBigIntegers(t *testing.T) {
	// A big integer travels to a process and back; a float field takes one,
	// an int field, a protobuf int64, does not.
	src := `
include Cameo
class Twice
  def go(n) = n * 2
end
class Account
  int :cents
  float :rate
end
p await(async(Twice.new, :go, 2**70))
p Account.new(rate: 2**64).rate
begin
  Account.new(cents: 2**64)
rescue RangeError => e
  puts e.message
end
`
	expectOutput(t, src, "2361183241434822606848\n1.8446744073709552e+19\nfield cents of Account takes an int, from -9223372036854775808 to 9223372036854775807; 18446744073709551616 does not fit\n")
}

func TestLazyStopsWhereItIsDone(t *testing.T) {
	// take ends an endless source after its last element, and the stop of
	// an inner pipeline does not end the outer one.
	src := `
n = 0
p (1..).lazy.map { |x| n += 1; x }.take(3).to_a, n
p (1..3).lazy.flat_map { |x| (x..).lazy.take(2) }.to_a
p (1..).lazy.zip(10..).take(2).to_a
`
	expectOutput(t, src, "[1, 2, 3]\n3\n[1, 2, 2, 3, 3, 4]\n[[1, 10], [2, 11]]\n")
}

func TestMethodMissing(t *testing.T) {
	// A call to a missing method goes to method_missing with its name, its
	// arguments and its block; super raises, and respond_to? asks
	// respond_to_missing?.
	src := `
class Ghost
  def method_missing(name, *args, &blk)
    name == :boo ? [args, blk.call] : super
  end
  def respond_to_missing?(name, all) = name == :boo
end
g = Ghost.new
p g.boo(1) { 2 }, g.respond_to?(:boo), g.respond_to?(:bar)
g.bar
`
	expectError(t, src, "undefined method 'bar' for an instance of Ghost (NoMethodError)")
	expectOutput(t, strings.TrimSuffix(src, "g.bar\n"), "[[1], 2]\ntrue\nfalse\n")
}

func TestSend(t *testing.T) {
	// send calls a method by name, a symbol or a string, with the
	// arguments, keywords and block it is given, and reaches method_missing
	// and top-level methods as a call written with that name does.
	expectOutput(t, `
class Greeter
  string :name
  def hi(greeting, punct: "!") = "#{greeting}, #{name}#{punct}"
  def method_missing(n, *args) = n == :wave ? "#{name} waves" : super
end
g = Greeter.new(name: "Ann")
p g.send(:hi, "Hello"), g.public_send("hi", "Hi", punct: "?"), g.__send__(:wave)
p 1.send(:+, 2), [1, 2].send(:map) { |x| x * 2 }
def helper(x) = x * 10
p send(:helper, 4), %i[upcase reverse].map { |m| "abc".send(m) }
`, "\"Hello, Ann!\"\n\"Hi, Ann?\"\n\"Ann waves\"\n3\n[2, 4]\n40\n[\"ABC\", \"cba\"]\n")
	// send rebinds no variable, so it refuses what a call on a temporary
	// refuses, and initialize, which runs only in new.
	for src, want := range map[string]string{
		"a = [1]\na.send(:push, 2)\n":             "send(:push): push changes its receiver",
		"p [2, 1].send(:sort!)\n":                 "send(:sort!): sort!: a bang method changes its receiver",
		"h = {a: 1}\nh.send(:[]=, :b, 2)\n":       "send(:[]=): []= is not supported",
		"p 1.send(:eval, \"1\")\n":                "send(:eval): eval is not supported",
		"class G\nend\nG.new.send(:initialize)\n": "send(:initialize): initialize runs only in new",
		"p 1.send(:nope)\n":                       "undefined method 'nope' for an instance of Integer",
		"p 1.send(1)\n":                           "1 is not a symbol nor a string (TypeError)",
		"p 1.send\n":                              "no method name given (ArgumentError)",
	} {
		expectError(t, src, want)
	}
}

func TestCameoIsIncluded(t *testing.T) {
	// async and await come from Cameo: a script includes it at the top
	// level, for every method, or in a class body, for that class, or
	// writes Cameo.async.
	expectError(t, "class W\n  def go = 1\nend\nasync W.new, :go\n", "async comes from the Cameo module: include Cameo, or call Cameo.async (NoMethodError)")
	expectError(t, "include Comparable\n", "include at the top level takes Cameo")
	expectOutput(t, `
class W
  def go(n) = n + 1
end
p Cameo.await(Cameo.async(W.new, :go, 1))
class Boss
  include Cameo
  def run = await(W.new, :go, 2)
end
p Boss.new.run
`, "2\n3\n")
	expectOutput(t, `
include Cameo
class W
  def go(n) = n * 10
  def relay(promise) = resolve(promise, await(self, :go, 4))
end
pending = promise
async W.new, :relay, pending
p await(pending)
`, "40\n")
}

func TestWorkerSeesTheTopLevelAtItsStart(t *testing.T) {
	// A worker sees the constants and top-level methods defined before its
	// async, not those defined after.
	expectOutput(t, `
include Cameo
def twice(n) = n * 2
LIMIT = 1
class W
  def go(n) = [twice(n), LIMIT, await(self, :nested, n)]
  def nested(n) = twice(n) + LIMIT
end
first = async W.new, :go, 1
LIMIT = 2
def twice(n) = n * 3
second = async W.new, :go, 1
third = async W.new, :go, 2
class W
  def go(n) = :redefined
end
p await([first, second, third])
`, "[[2, 1, 3], [3, 2, 5], [6, 2, 8]]\n")
}

func TestWorkerSeesTopLevelChangesAfterAnAsyncInAClassBody(t *testing.T) {
	// A class whose body starts an async, and a top-level alias, still
	// reach the workers started after them.
	expectOutput(t, `
include Cameo
def a = 1
class H
  def one = 1
end
class U
  def go = [Foo.x, b]
end
class Foo
  def self.x = 2
  P = async H.new, :one
end
alias b a
p await(async U.new, :go)
`, "[2, 1]\n")
}

func TestArgumentsCrossAsTheEnvelopeCarriesThem(t *testing.T) {
	// What a worker gets is what the envelope reads back: a symbol is a
	// string, an instance has its class's fields. What it cannot carry is
	// a TypeError at the async call.
	expectOutput(t, `
include Cameo
class Echo
  def go(*args) = args
end
class Point
  int64 :x
  string :tag
end
p await(Echo.new, :go, nil, true, 7, 2**70, 1.5, "s", "\xff".b, :sym, [1, [2, "t"], :u], {a: 1}, Point.new(x: 1), Echo.new)
`, "[nil, true, 7, 1180591620717411303424, 1.5, \"s\", \"\\xFF\", \"sym\", [1, [2, \"t\"], \"u\"], {\"a\" => 1}, Point(x: 1, tag: nil), Echo()]\n")
	expectError(t, "include Cameo\nclass Echo\n  def go(x) = x\nend\nawait Echo.new, :go, Echo\n", "a Class cannot be passed to async (TypeError)")
	expectError(t, "include Cameo\nanon = Class.new { def go = 1 }\nawait anon.new, :go\n", "has an invalid nested name")
	expectError(t, "include Cameo\nclass Echo\n  def go(x) = x\nend\nawait Echo.new, :go, Object.new\n", "Object is not a class with fields (TypeError)")
}

func TestAsyncRejectsWhatTheEnvelopeRejects(t *testing.T) {
	// A string that is not valid UTF-8 cannot cross, on its own, in a list,
	// or as a worker's result.
	echo := "include Cameo\nclass Echo\n  def go(*args) = args\n  def bad = \"\xff\"\nend\n"
	for _, call := range []string{`await Echo.new, :go, "\xff"`, `await Echo.new, :go, ["\xff"]`, `await Echo.new, :go, ["\xff", :a]`, `await Echo.new, :bad`} {
		expectError(t, echo+call+"\n", "invalid UTF-8")
	}
}

func TestWorkersEncodeASharedFieldlessInstance(t *testing.T) {
	// The parent and a worker both encode the instance they share, inside
	// a Hash; go test -race reports it when they write one cached message.
	expectOutput(t, `
include Cameo
class Tag
end
class Echo
  def go(t) = await(Echo.new, :wrap, {k: t})
  def wrap(h) = h
end
t = Tag.new
jobs = (1..8).map { [async(Echo.new, :go, t), async(Echo.new, :wrap, {k: t})] }
p await(jobs.flatten).uniq
`, "[{\"k\" => Tag()}]\n")
}

func TestRunReportsFailuresNobodyAwaited(t *testing.T) {
	// The run waits for every call async started, a worker's included,
	// and names each failure nobody awaited, in start order.
	expectOutput(t, `
include Cameo
class W
  def boom(tag) = raise(ArgumentError, tag)
  def fan = async(self, :boom, "inner")
end
async W.new, :boom, "first"
seen = async W.new, :boom, "awaited"
begin
  await seen
rescue ArgumentError
end
await W.new, :fan
async W.new, :boom, "last"
`, "cameo: promise 1 (boom) failed and was never awaited: first (ArgumentError)\n"+
		"cameo: promise 4 (boom) failed and was never awaited: inner (ArgumentError)\n"+
		"cameo: promise 5 (boom) failed and was never awaited: last (ArgumentError)\n")
}

func TestAPromiseEndsWithTheMethodThatStartedIt(t *testing.T) {
	// A promise async started is forgotten once the method that started
	// it has returned and it was awaited, there or later.
	src := `
include Cameo
class W
  def go = 7
  def awaited = await(async(self, :go)) + 0
  def kept
    p = async(self, :go)
    await p
    p
  end
  def started = async(self, :go)
end
p W.new.awaited
later = W.new.started
p await(later)
begin
  await later
rescue ArgumentError => e
  p e.message
end
begin
  await W.new.kept
rescue ArgumentError => e
  p e.message
end
`
	expectOutput(t, src, "7\n7\n\"no promise 2\"\n\"no promise 3\"\n")
}

func TestAwaitedPromisesLeaveTheTable(t *testing.T) {
	prog, err := syntax.Parse("t.cameo", `
include Cameo
class Fib
  def of(n)
    return 1 if n < 2
    parts = await([async(self, :of, n - 1), async(self, :of, n - 2)])
    parts[0] + parts[1]
  end
end
p Fib.new.of(12)
`)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	in := New(&out, &out)
	if err := in.Run("t.cameo", prog); err != nil {
		t.Fatal(err)
	}
	if out.String() != "233\n" {
		t.Fatalf("output %q", out.String())
	}
	if started, left := in.promises.next.Load(), in.promises.len(); started < 400 || left != 0 {
		t.Fatalf("%d promises started, %d left in the table", started, left)
	}
}

func TestPromisesOfAMethodWhoseArgumentsFailLeaveTheTable(t *testing.T) {
	// A default argument awaited a call before a missing keyword failed the
	// method; its promise goes when the method does.
	prog, err := syntax.Parse("t.cameo", `
include Cameo
class W
  def one = 1
  def m(a = await(async(self, :one)), b:) = a
end
10.times do
  W.new.m
rescue ArgumentError
end
`)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	in := New(&out, &out)
	if err := in.Run("t.cameo", prog); err != nil {
		t.Fatal(err)
	}
	if started, left := in.promises.next.Load(), in.promises.len(); started != 10 || left != 0 {
		t.Fatalf("%d promises started, %d left in the table: %s", started, left, out.String())
	}
}

func TestCallsThatWaitDoNotHoldUpTheQueue(t *testing.T) {
	// More calls than there are cores wait for a promise that only the
	// last call started settles; each must still get to run.
	n := 4*runtime.GOMAXPROCS(0) + 2
	expectOutput(t, fmt.Sprintf(`
include Cameo
class W
  def wait_for(gate) = await(gate)
  def open(gate) = resolve(gate, :open)
end
gate = promise
waiting = (1..%d).map { async(W.new, :wait_for, gate) }
async W.new, :open, gate
p await(waiting).uniq
`, n), "[:open]\n")
}

func TestCallsThatNeverEndDoNotHoldUpTheQueue(t *testing.T) {
	// More calls than there are cores keep busy for two seconds; a call
	// started after them must not wait until they end.
	n := 2*runtime.GOMAXPROCS(0) + 1
	expectOutput(t, fmt.Sprintf(`
include Cameo
def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
class W
  def spin(seconds)
    t0 = now
    x = 0
    x += 1 while now - t0 < seconds
    x
  end
  def finish(gate) = resolve(gate, :done)
end
t0 = now
%d.times { async(W.new, :spin, 2) }
gate = promise
async W.new, :finish, gate
p await(gate), now - t0 < 1
`, n), ":done\ntrue\n")
}

func TestCameoRunnerIsCameo(t *testing.T) {
	// Cameo::Runner is Cameo for the class that includes it, the module a
	// Ruby program includes too; the top level refuses it. require
	// "cameo/runner" is the library that gives Ruby the module: cameo has
	// it built in.
	expectOutput(t, `
p require("cameo/runner"), require("cameo/runner")
class W
  def go(n) = n * 2
end
class Boss
  include Cameo::Runner
  def run = await(W.new, :go, 5)
end
p Boss.new.run, Cameo::Runner.class
`, "true\nfalse\n10\nModule\n")
	expectError(t, "include Cameo::Runner\n", "include Cameo::Runner belongs in the class that runs the work; at the top level include Cameo")
}

func TestPromises(t *testing.T) {
	// A promise is an Integer that resolve or fail settles once, from the
	// script or a worker; await takes one or a list.
	expectOutput(t, `
include Cameo
class Courier
  def deliver(promise, v) = resolve(promise, v)
  def lose(promise) = fail(promise, KeyError, "lost")
end
a = promise
b = promise
async Courier.new, :deliver, a, [1, {x: 2}]
async Courier.new, :deliver, b, "two"
p a, b, await([a, b])
c = promise
async Courier.new, :lose, c
begin
  await [a, c]
rescue KeyError => e
  p e.message
end
d = promise
fail d, "plain"
begin
  await d
rescue => e
  p e.class
end
begin
  fail "not a promise"
rescue => e
  p e.message
end
`, "1\n2\n[[1, {\"x\" => 2}], \"two\"]\n\"lost\"\nRuntimeError\n\"not a promise\"\n")
	for src, want := range map[string]string{
		"include Cameo\np = promise\nresolve(p, 1)\nresolve(p, 2)\n": "promise 1 is settled already (ArgumentError)",
		"include Cameo\nawait 7\n":                                   "no promise 7 (ArgumentError)",
		"include Cameo\nawait \"x\"\n":                               "a promise is the Integer async or promise returned, not an instance of String (TypeError)",
		"include Cameo\nresolve(promise, -> { 1 })\n":                "resolve got a Proc, which cannot be passed back (TypeError)",
	} {
		expectError(t, src, want)
	}
}

func TestFileIsTheScript(t *testing.T) {
	// __FILE__ is the script's path, as it was given, which File's path
	// functions take apart.
	expectOutput(t, "p __FILE__, File.extname(__FILE__), File.basename(__FILE__, \".cameo\")\n", "\"t.cameo\"\n\".cameo\"\n\"t\"\n")
}

func TestTimeCrossesAsync(t *testing.T) {
	// A time travels to a process and back with its instant and offset.
	src := `
include Cameo
class Later
  def go(t) = t + 60
end
t = Time.new(2021, 8, 22, 9, 0, 0, "+09:00")
p await(async(Later.new, :go, t))
p await(async(Later.new, :go, t.getutc))
`
	expectOutput(t, src, "2021-08-22 09:01:00 +0900\n2021-08-22 00:01:00 UTC\n")
}

func TestRandomIsSeedable(t *testing.T) {
	// The same seed gives the same draws, in the script and in its
	// processes; srand returns the previous seed.
	src := `
include Cameo
class Draw
  def go(n) = (1..n).map { rand(1000) }
end
def draws
  srand(42)
  local = [rand, rand(10), rand(1..6), [1, 2, 3, 4, 5].shuffle, [1, 2, 3, 4, 5].sample(2)]
  remote = await(async(Draw.new, :go, 5))
  [local, remote]
end
first = draws
second = draws
puts first == second
puts srand(7) == 42
puts srand(9) == 7
`
	expectOutput(t, src, "true\ntrue\ntrue\n")
}

func TestSortsWithoutDispatch(t *testing.T) {
	// Collections of one builtin kind sort and find their extremes without
	// calling <=>; a NaN, a mix of kinds or a block takes the general path,
	// which raises where Ruby does.
	src := `
p [3.5, -0.0, 0.0, -2.0, 1e300, -Float::INFINITY].sort
p ["b", "a".b, "é", "a", "", "B", "ab"].sort
p ["b", "a".b, "é"].sort.map(&:encoding)
p [1, 2.5, 0].sort
p ["b", "a"].sort { |x, y| y <=> x }
begin
  [1.0, Float::NAN].sort
rescue ArgumentError
  puts "nan"
end
begin
  ["b", :a].sort
rescue ArgumentError
  puts "mixed"
end
p [2.5, -1.0, 7.0].max, ["b", "a", "c"].min, [3, 1, 2].minmax, [1.0, 1, 0.5].min
p Set[3.0, 1.5].sort, Set["b", "a"].max, (1..4).max(2)
begin
  [1.0, Float::NAN].max
rescue ArgumentError
  puts "nan"
end
`
	expectOutput(t, src, `[-Infinity, -2.0, -0.0, 0.0, 3.5, 1.0e+300]
["", "B", "a", "a", "ab", "b", "é"]
[#<Encoding:ASCII-8BIT>, #<Encoding:UTF-8>, #<Encoding:UTF-8>]
[0, 1, 2.5]
["b", "a"]
nan
mixed
7.0
"a"
[1, 3]
0.5
[1.5, 3.0]
"b"
[4, 3]
nan
`)
}

func TestClockGettime(t *testing.T) {
	// The monotonic clock only goes forward; a unit scales the reading.
	src := `
a = Process.clock_gettime(Process::CLOCK_MONOTONIC)
b = Process.clock_gettime(Process::CLOCK_MONOTONIC)
p a.class, b >= a
p Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond).class
p Process.clock_gettime(Process::CLOCK_REALTIME, :second) > 1_700_000_000
begin
  Process.clock_gettime(99)
rescue ArgumentError => e
  puts e.message
end
begin
  Process.clock_gettime(Process::CLOCK_MONOTONIC, :bogus)
rescue ArgumentError => e
  puts e.message
end
`
	expectOutput(t, src, "Float\ntrue\nInteger\ntrue\nInvalid argument - clock_gettime(99)\nunexpected unit: bogus\n")
}

func TestSecondDefReplacesTheFirst(t *testing.T) {
	// A second def of a name replaces the first, at the top level and in a
	// class body; a call made before it ran saw the first. A loop around a
	// def evaluates the one definition again, which changes nothing.
	expectOutput(t, "def f = 1\np f\ndef f = 2\np f\n", "1\n2\n")
	expectOutput(t, "3.times do\n  def f = 1\nend\np f\n", "1\n")
	expectOutput(t, "class A\n  def x = 1\n  def y = x\n  def x = 2\nend\np A.new.y\n", "2\n")
	expectError(t, "class P\n  int :x\n  def x = 1\nend\n", "x is a field of P")
}

func TestRangeLastCountsFromTheEnd(t *testing.T) {
	// The last n of an integer range come from its end, without the
	// elements before them, and fail as Ruby's do.
	src := `
p (1..1_000_000_000_000).last(3), (1...10).last(2), (5..1).last(2), (1..3).last(10), (1..3).last(0)
p (-5..-1).last(2), ("a".."e").last(2)
begin
  (1..3).last(-1)
rescue ArgumentError => e
  puts e.message
end
begin
  (1..).last(2)
rescue RangeError => e
  puts e.message
end
`
	expectOutput(t, src, "[999999999998, 999999999999, 1000000000000]\n[8, 9]\n[]\n[1, 2, 3]\n[]\n[-2, -1]\n[\"d\", \"e\"]\nnegative array size\ncannot get the last element of endless range\n")
}

func TestFieldNumbers(t *testing.T) {
	expectOutput(t, `class P
  int :x
  int :y, number: 1
  int :z, number: 4096
end
puts P.proto
`, `message P {
  optional int64 x = 64843;
  optional int64 y = 1;
  optional int64 z = 4096;
}
`)
	expectOutput(t, "class P\n  int :y\n  int :x\nend\nputs P.proto\n",
		"message P {\n  optional int64 y = 64608;\n  optional int64 x = 64843;\n}\n")
	expectError(t, "class P\n  int :x, number: 2\n  int :y, number: 2\nend\n", "field y: number 2 is field x's")
	expectError(t, "class P\n  reserved 2\n  int :x, number: 2\nend\n", "field x: number 2 is reserved in P")
}

// len returns, for tests, the number of promises in the table.
func (t *promises) len() int {
	n := 0
	for i := range t.shards {
		s := &t.shards[i]
		s.mu.Lock()
		n += len(s.byID)
		s.mu.Unlock()
	}
	return n
}

func TestRubyVersionConstants(t *testing.T) {
	expectOutput(t, `p RUBY_ENGINE
p RUBY_VERSION
p RUBY_PATCHLEVEL
p RUBY_ENGINE_VERSION == RUBY_DESCRIPTION.split(" ")[1]
p RUBY_DESCRIPTION.start_with?("cameo ")
p RUBY_DESCRIPTION.end_with?("+ruby #{RUBY_VERSION} [#{RUBY_PLATFORM}]")
p RUBY_PLATFORM.match?(/\A[a-z0-9_]+-[a-z]+/)
p RUBY_COPYRIGHT
p [RUBY_ENGINE, RUBY_VERSION, RUBY_PLATFORM, RUBY_RELEASE_DATE, RUBY_REVISION].all?(&:frozen?)
`, `"cameo"
"3.4.0"
0
true
true
true
true
"cameo - Copyright (C) 2026 the cameo authors (MIT License)"
true
`)
}

func TestRubyVersionConstantsCannotBeReassigned(t *testing.T) {
	expectError(t, "RUBY_VERSION = \"9.9.9\"\n", "cannot reassign the builtin constant RUBY_VERSION")
	expectError(t, "RUBY_PATCHLEVEL = 1\n", "cannot reassign the builtin constant RUBY_PATCHLEVEL")
	expectOutput(t, "class A\n  RUBY_VERSION = \"1.8\"\n  def self.v = RUBY_VERSION\nend\nputs A.v\nputs RUBY_VERSION\n", "1.8\n3.4.0\n")
}
