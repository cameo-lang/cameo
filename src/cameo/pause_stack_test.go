package cameo

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
)

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
