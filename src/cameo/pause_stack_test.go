package cameo

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
)

// everyPauseLands proves a run of src pauses once for each call of
// Test.pause: no pause is held off until a builtin returns, where one
// pause would stand for the calls made inside the builtin.
func everyPauseLands(t *testing.T, src string) {
	t.Helper()
	calls := 0
	var out bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) {
		calls++
		return nil, nil
	})
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	pauses := 0
	prog, _ = pausing(t, src)
	for {
		err := prog.Run()
		if !errors.Is(err, ErrPaused) {
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		pauses++
		dump, err := prog.Dump()
		if err != nil {
			t.Fatal(err)
		}
		cont, err := Resume(dump.Data, &out, &out)
		if err != nil {
			t.Fatal(err)
		}
		cont.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) {
			cont.Pause()
			return nil, nil
		})
		prog = cont
	}
	if pauses != calls || calls == 0 {
		t.Fatalf("the script called Test.pause %d times and paused %d times", calls, pauses)
	}
}

// stackOf reads the activations of a dump.
func stackOf(t *testing.T, dump *Dump) *cameopb.Dump {
	t.Helper()
	d := &cameopb.Dump{}
	if err := proto.Unmarshal(dump.Data, d); err != nil {
		t.Fatal(err)
	}
	return d
}

// A run pauses inside a method, some calls deep, and resumes there: the
// locals of every call, the values of the expressions in flight and the
// loop it is in come back.
func TestPauseInsideAMethodResumesThere(t *testing.T) {
	const src = `class Account
  int :balance
  def deposit(n) = Account.new(balance: balance + checked(n))
  def checked(n)
    raise ArgumentError, "negative" if n < 0
    Test.pause
    n
  end
end
def total(amounts)
  acc = Account.new(balance: 0)
  i = 0
  while i < amounts.size
    acc = acc.deposit(amounts[i])
    puts "after #{amounts[i]}: #{acc.balance}"
    i += 1
  end
  acc.balance
end
puts "start"
puts "total #{total([5, 10, 20])}" + " done"
puts "end"
`
	dump := sameAsWhole(t, src)
	d := stackOf(t, dump)
	var kinds []string
	for _, a := range d.Stack {
		kinds = append(kinds, a.Kind.String())
	}
	// The statement, total, deposit, checked.
	if got := strings.Join(kinds, " "); got != "STATEMENT METHOD METHOD METHOD" {
		t.Fatalf("the dump's activations are %s", got)
	}
	if dump.Line != 7 {
		t.Fatalf("paused before line %d, want 7", dump.Line)
	}
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	want := "paused: before line 7\ninside:\n  Account#checked, line 7\n  Account#deposit, line 3\n  total, line 14\n"
	if !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it:\n%s", text, want)
	}
}

// A run pauses inside a block a method yields to, and inside the clauses
// of a begin: the block's return, break and next still find their call
// after the resume, a rescue clause still has its exception, and an
// ensure clause still knows what it interrupted.
func TestPauseInsideBlocksAndClausesResumesThere(t *testing.T) {
	sameAsWhole(t, `def each3
  yield 1
  yield 2
  yield 3
ensure
  puts "each3 done"
end
def first_over(limit)
  each3 do |x|
    Test.pause
    return x if x > limit
  end
  nil
end
p first_over(1)
r = each3 do |x|
  Test.pause
  next x * 2 if x == 1
  break x * 100 if x == 2
end
p r
def guarded
  yield
rescue ArgumentError => e
  Test.pause
  puts "rescued #{e.message}"
  raise
ensure
  Test.pause
  puts "ensure"
end
begin
  guarded { raise ArgumentError, "bad" }
rescue => e
  p [e.class, e.message]
end
def leaving
  each3 do |x|
    begin
      return "left at #{x}" if x == 2
    ensure
      Test.pause
      puts "ensure #{x}"
    end
  end
end
p leaving
def retried
  tries = 0
  begin
    tries += 1
    raise "again" if tries < 3
    "done after #{tries}"
  rescue
    Test.pause
    retry
  end
end
p retried
`)
}

// What a statement in flight holds besides its calls comes back: a
// string half interpolated, the subject of a case, the arguments already
// evaluated, a lambda and the frame it returns from, a class as a value,
// the top-level self.
func TestPauseKeepsTheExpressionsInFlight(t *testing.T) {
	sameAsWhole(t, `def mark(x)
  Test.pause
  x
end
class Point
  int :x, :y
  def self.origin = new(x: mark(0), y: mark(0))
  def to_s = "(#{mark(x)}, #{mark(y)})"
end
def pick(a, b, c) = [a, b, c]
puts "a #{mark(1)} b #{mark(2)} c"
p pick(mark(:a), [mark(1), mark(2)], {k: 1}.merge({"j" => mark(3)}))
p(case mark(5)
  when mark(4) then "four"
  when mark(5) then "five"
  end)
puts Point.origin, Point.new(x: 1, y: 2)
add = ->(a) { return a + mark(1) }
p add.call(mark(2)), add.(3)
def me = mark(self)
p me == self, mark(Point) == Point
i = 0
while mark(i) < 2
  i += 1
end
p i
`)
}

// A symbol is a symbol after a resume: in a variable, as the key of a
// Hash, in what a Proc captured, in a constant of a class. In a message
// for another program it is a String, which a dump is not.
func TestASymbolInADumpStaysASymbol(t *testing.T) {
	sameAsWhole(t, `class Light
  STATES = [:red, :green]
  def self.after(s) = STATES[(STATES.index(s) + 1) % 2]
end
mode = :red
table = {red: "stop", green: "go"}
check = -> { [mode, mode == :red, table[mode]] }
Test.pause
p mode, mode == :red, table[mode], table.keys, check.call
p Light.after(mode), Light::STATES
p from_value_proto(mode.to_value_proto)
`)
}

// A run pauses inside the block of a builtin that is a stepper, and the
// builtin goes on where it was: times, upto and downto with the number
// they came to, loop with nothing. A break out of the block and a
// StopIteration still end the builtin after the resume.
func TestPauseInsideTheBlockOfAStepper(t *testing.T) {
	const src = `def squares(n)
  n.times do |i|
    Test.pause if i == 2
    puts i * i
  end
end
p squares(5)
r = 3.times { |i| Test.pause }
p r
1.upto(4) { |i| Test.pause if i == 3; puts "up #{i}" }
p 10.downto(7) { |i| Test.pause; break i * 2 if i == 8 }
big = 2**70
big.upto(big + 2) { |i| Test.pause; p i - big }
srand(7)
r = loop do
  Test.pause
  x = rand(5)
  raise StopIteration if x == 0
  puts "tick #{x}"
end
p r
p (loop { Test.pause; break :out })
3.times do |i|
  2.times do |j|
    Test.pause
    puts "#{i}.#{j}"
  end
end
p 2.times.to_a, 4.times.map { |i| i + 1 }
`
	dump := sameAsWhole(t, src)
	everyPauseLands(t, src)
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	// The first pause: in the block of times, in squares.
	want := "paused: before line 4\ninside:\n  a block in squares, line 4\n  Integer#times\n  squares, line 2\n"
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
	if !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it:\n%s", text, want)
	}
}

// A run pauses inside the block of a walk over a List or a Range, and
// the walk goes on with what it had gathered: each and each_index,
// each_with_index, each_with_object, map, flat_map, filter_map, select,
// reject, partition and find. A Hash and a Set have walks of their own
// for most of these, which still hold a pause off; the second script
// goes through them too, for what they compute.
func TestPauseInsideTheBlockOfAWalk(t *testing.T) {
	// Every one of these pauses inside the block: none waits for the
	// walk to return.
	everyPauseLands(t, `def mark(x)
  Test.pause
  x
end
list = [3, 1, 2]
list.each { |x| mark(x) }
list.each_index { |i| mark(i) }
list.each_with_index { |x, i| mark(x) }
list.each_with_object([]) { |x, memo| memo + [mark(x)] }
list.map { |x| mark(x) }
list.collect { |x| mark(x) }
list.flat_map { |x| [mark(x)] }
list.filter_map { |x| mark(x) }
list.select { |x| mark(x) > 1 }
list.filter { |x| mark(x) > 1 }
list.find_all { |x| mark(x) > 1 }
list.reject { |x| mark(x) > 1 }
list.partition { |x| mark(x) > 1 }
list.find { |x| mark(x) == 2 }
list.detect { |x| mark(x) == 9 }
(1..3).map { |x| mark(x) }
list.map { |x| [x].map { |y| 2.times { mark(y) } } }
`)
	sameAsWhole(t, `def mark(x)
  Test.pause
  x
end
list = [3, 1, 2]
list.each { |x| puts mark(x) }
list.each_index { |i| puts mark(i) }
p list.each_with_index { |x, i| puts "#{mark(x)} at #{i}" }
p list.each_with_object([]) { |x, memo| memo + [mark(x) * 2] }
p list.map { |x| mark(x) + 1 }, list.collect { |x| mark(x).to_s }
p list.flat_map { |x| [mark(x), [x]] }, list.filter_map { |x| mark(x) * 2 if x > 1 }
p list.select { |x| mark(x) > 1 }, list.reject { |x| mark(x) > 1 }, list.filter { |x| mark(x).odd? }
p list.partition { |x| mark(x) > 1 }
p list.find { |x| mark(x) < 3 }, list.detect { |x| mark(x) > 5 }
p (1..4).map { |x| mark(x) * x }, (1..4).select { |x| mark(x).even? }
p({a: 1, b: 2}.map { |k, v| [mark(k), v] }, {a: 1, b: 2}.find { |k, v| mark(v) == 2 })
p Set[1, 2].map { |x| mark(x) + 10 }
p list.map { |x| [x, x + 1].map { |y| mark(y) * 10 } }
p list.each { |x| break mark(x) if x == 1 }
p list.map { |x| next 0 if mark(x) == 1; x }
r = list.map do |x|
  begin
    raise "odd" if mark(x).odd?
    x
  rescue => e
    mark(e.message)
  end
end
p r
p list.each_with_index.map { |x, i| x * i }, list.map.with_index { |x, i| [i, x] }
p list.each_slice(2).map { |pair| mark(pair) }
`)
}

// A run pauses inside the block of the walks a Hash, a Set and a Range
// have of their own: each over all three, a Hash's each_pair, each_key,
// each_value, select, reject and transform_values, a Set's select and
// reject. A Range's each keeps the integer it came to, also when the
// range has no end or is past 64 bits.
func TestPauseInsideTheWalksOfAHashASetAndARange(t *testing.T) {
	const src = `def mark(x)
  Test.pause
  x
end
h = {a: 1, "b" => 2, 3 => [4]}
h.each { |k, v| p [mark(k), v] }
h.each_pair { |k, v| p [k, mark(v)] }
h.each_key { |k| p mark(k) }
h.each_value { |v| p mark(v) }
p h.select { |k, v| mark(v) != 2 }, h.filter { |k, v| mark(k) == :a }
p h.reject { |k, v| mark(v) == 2 }
p h.transform_values { |v| [mark(v)] }
s = Set[3, 1, 2]
s.each { |x| p mark(x) }
p s.select { |x| mark(x) > 1 }, s.reject { |x| mark(x) > 1 }, s.filter { |x| mark(x).odd? }
(1..3).each { |i| p mark(i) }
(1...3).each { |i| p mark(i) }
p (5..).each { |i| break mark(i) * 2 if i == 7 }
big = 2**64
(big..big + 1).each { |i| p mark(i) - big }
p (1..0).each { |i| mark(i) }, (3..1).each { |i| mark(i) }
p h.each { |k, v| break mark(k) }
`
	everyPauseLands(t, src)
	sameAsWhole(t, src+`("a".."c").each { |c| p mark(c) }
p (1..3).each.map { |i| i * 2 }, h.each_key.to_a, s.each.to_a
p h.each_with_index.map { |(k, v), i| [k, i] }
`)
}

// A run pauses inside a lambda or a proc that is called as a value, with
// call, () or [], and resumes there: a lambda's return still returns
// from the lambda, a proc's from the method it was written in, and a
// lambda that calls itself is as deep as it was.
func TestPauseInsideAProcCalledAsAValue(t *testing.T) {
	const src = `def mark(x)
  Test.pause
  x
end
double = ->(x) { mark(x) * 2 }
p double.call(1), double.(2), double[3]
add = proc { |a, b| mark(a) + b }
p add.call(1, 2), add.yield(3, 4)
fact = ->(n) { n <= 1 ? mark(1) : n * fact.(n - 1) }
p fact.(5)
early = ->(x) { return mark(x) if x > 1; :small }
p early.(1), early.(2)
def apply(x, &blk) = blk.call(mark(x))
p apply(3) { |v| mark(v) + 1 }
def finder(list)
  check = proc { |x| return mark(x) if x.even? }
  list.each { |x| check.call(x) }
  :none
end
p finder([1, 3, 4, 5]), finder([1, 3])
compose = ->(f, g) { ->(x) { g.(f.(mark(x))) } }
p compose.(double, ->(y) { mark(y) + 1 }).(5)
case 4
when ->(n) { mark(n).even? } then puts "even"
end
p [1, 2, 3].map(&double), [1, 2].each_with_object([]) { |x, memo| memo + [double.(x)] }
`
	everyPauseLands(t, src)
	sameAsWhole(t, src)
}

// A run that paused goes on when it is run again, from where it paused:
// what a host does with a script it could not dump. Here the block of
// loop holds a StringIO, which no dump carries.
func TestARunGoesOnWhenItIsRunAgain(t *testing.T) {
	const src = `require "stringio"
def drain(io)
  loop do
    c = io.getc
    break if c.nil?
    Test.pause if c == "b"
    puts "read #{c}"
  end
  "drained"
end
puts drain(StringIO.new("abc"))
x = 1
Test.pause
puts x + 1
`
	prog, out := pausing(t, src)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if _, err := prog.Dump(); err == nil || !strings.Contains(err.Error(), "a StringIO cannot be serialised") {
		t.Fatalf("Dump of a run inside a block that holds a StringIO: %v", err)
	}
	// Inside the statement, then between two statements, then to the end.
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("the second Run returned %v, want ErrPaused", err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), whole(t, src); got != want {
		t.Fatalf("paused and run again the script printed\n%s\nand in one run\n%s", got, want)
	}
}

// A statement that never ends pauses, which before it never did.
func TestALoopThatNeverEndsPauses(t *testing.T) {
	prog, out := pausing(t, `i = 0
while true
  i += 1
  Test.pause if i == 1000
  puts "past the pause" if i == 1001
  break if i == 1002
end
puts i
`)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if out.String() != "" {
		t.Fatalf("the script printed %q before its pause", out.String())
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "paused: before line 5\n"; !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it: %s", text, want)
	}
	if want := "variables:\n  i = 1000\n"; !strings.Contains(text, want) {
		t.Fatalf("inspect:\n%s\nwant in it: %s", text, want)
	}
}

// A value in flight that the envelope cannot carry is named when the run
// pauses, as a variable is.
func TestPauseNamesWhatAStackCannotCarry(t *testing.T) {
	prog, _ := pausing(t, `def wait(e)
  Test.pause
  e.size
end
p wait([1, 2].each)
`)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(dump.Missing, ";")
	if !strings.Contains(got, "e of scope 1 (an Enumerator cannot be serialised)") {
		t.Fatalf("missing: %s", got)
	}
}
