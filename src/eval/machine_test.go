package eval

import (
	"reflect"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// A machine that is kept for the next call holds nothing of the call it
// ran: no value, no cursor, no frame.
func TestAReleasedMachineHoldsNothing(t *testing.T) {
	in := New(nil, nil)
	m := in.machine()
	m.kind, m.fr, m.env = actMethod, &Frame{}, NewEnv(nil)
	m.push(object.NewString("kept alive", object.UTF8))
	m.push(object.Int(1))
	m.pop()
	m.cursors = append(m.cursors, cursor{pending: ErrStopped})
	m.cursors = m.cursors[:0]
	in.release(m)

	again := in.machine()
	assert.True(t, again == m)
	assert.True(t, again.in == in)
	assert.True(t, again.fr == nil && again.env == nil && again.kind == actList)
	assert.Equals(t, len(again.stack), 0)
	assert.Equals(t, len(again.cursors), 0)
	for _, v := range again.stack[:cap(again.stack)] {
		assert.True(t, v.IsZero())
	}
	for _, c := range again.cursors[:cap(again.cursors)] {
		assert.True(t, c.pending == nil)
	}
}

// A machine that ran a call and was released is a new machine but for
// the capacity it keeps, whichever fields a machine has: release sets
// each field by itself, and one it forgot would show here.
func TestAReleasedMachineIsAsNew(t *testing.T) {
	in := New(nil, nil)
	out, err := run(t, "def f(a, b, c, d, e, g)\n  x = begin\n    a + b\n  ensure\n    c\n  end\n  i = 0\n  while i < 2\n    i += 1\n  end\n  x + d + e + g\nend\np f(1, 2, 3, 4, 5, 6)\np [1, 2].map { |v| v + 1 }\np lambda { |v| v }.call(3)\n")
	assert.NoError(t, err)
	assert.Equals(t, out, "18\n[2, 3]\n3\n")
	m := in.machine()
	m.unit, m.syms, m.env, m.fr = &cameopb.Unit{}, &unitNames{}, &m.own.env, &m.own.frame
	m.own.frame = Frame{Method: &object.Method{}, Lambda: true, Self: object.Int(1)}
	m.own.env.lay(newLayout(syms("a", "b", "c", "d", "e", "f")))
	m.own.env.parent = NewEnv(nil)
	m.stack, m.cursors = append(m.stack, object.Int(1)), append(m.cursors, cursor{next: 1})
	m.bases, m.tries = append(m.bases, 1), append(m.tries, try{base: 1, pending: ErrStopped})
	m.flat, m.pc, m.flatBody, m.again = &flatCode{}, 3, true, true
	m.kind, m.proc, m.site, m.debugged, m.stepper = actBlock, &Proc{}, &cameopb.Code{}, true, &stepping{}
	m.result, m.wait, m.inGo, m.passing, m.row, m.sofar = object.Int(2), &awaiting{}, 1, true, 1, []byte("so far")
	// Every field is set to something: a field added to machine is to
	// be set here, and reset in release. held is not: release leaves a
	// held machine alone, so one it releases has it unset.
	set := reflect.ValueOf(m).Elem()
	for i := 0; i < set.NumField(); i++ {
		if name := set.Type().Field(i).Name; name != "spare" && name != "held" {
			assert.True(t, !set.Field(i).IsZero(), name+" is not set by the test")
		}
	}
	m.stepper = nil
	in.release(m)
	assert.True(t, in.free[len(in.free)-1] == m)

	fresh := &machine{in: in}
	got, want := reflect.ValueOf(m).Elem(), reflect.ValueOf(fresh).Elem()
	for i := 0; i < got.NumField(); i++ {
		name := got.Type().Field(i).Name
		switch name {
		case "stack", "cursors", "bases", "tries", "spare":
			assert.Equals(t, got.Field(i).Len(), 0)
		default:
			assert.True(t, reflect.DeepEqual(reflect.NewAt(got.Field(i).Type(), got.Field(i).Addr().UnsafePointer()).Elem().Interface(), reflect.NewAt(want.Field(i).Type(), want.Field(i).Addr().UnsafePointer()).Elem().Interface()), name+" is not reset")
		}
	}
	// The slots of the scope, more than it holds itself, are kept.
	assert.Equals(t, cap(m.spare), 6)
	for _, v := range m.spare[:cap(m.spare)] {
		assert.True(t, v.IsZero())
	}
	for _, tr := range m.tries[:cap(m.tries)] {
		assert.True(t, tr.pending == nil)
	}
}

// A machine whose stack grew large is not kept, and no more machines are
// kept than maxFree.
func TestMachinesKeptAreBounded(t *testing.T) {
	in := New(nil, nil)
	big := in.machine()
	big.stack = make([]object.Value, 0, maxKeptStack+1)
	in.release(big)
	assert.Equals(t, len(in.free), 0)

	for i := 0; i < maxFree+10; i++ {
		in.release(&machine{in: in})
	}
	assert.Equals(t, len(in.free), maxFree)
}

// Calls that run on reused machines return what they returned on fresh
// ones: recursion deeper than the machines kept, blocks, lambdas, a
// method left by an exception and one left by a break.
func TestCallsOnReusedMachines(t *testing.T) {
	out, err := run(t, `def fib(n) = n < 3 ? 1 : fib(n - 1) + fib(n - 2)
def down(n) = n == 0 ? 0 : 1 + down(n - 1)
def failing(n)
  raise "at #{n}" if n == 0
  failing(n - 1)
end
def each_twice
  yield 1
  yield 2
end
def first_over(limit)
  [1, 5, 9].each { |x| return x if x > limit }
  nil
end
puts fib(20)
puts down(1000)
puts down(1000)
begin
  failing(50)
rescue => e
  puts e.message
end
puts fib(15)
each_twice { |x| puts x * 10 }
add = ->(a, b) { a + b }
puts add.(1, 2), add.(3, 4)
puts first_over(4)
puts [3, 1, 2].sort { |a, b| b <=> a }.inspect
puts "a" + "b", 2.5 * 2, 7 / 2, 2**70
`)
	assert.NoError(t, err)
	assert.Equals(t, out, "6765\n1000\n1000\nat 0\n610\n10\n20\n3\n7\n5\n[3, 2, 1]\nab\n5.0\n3\n1180591620717411303424\n")
}

// Calls that keep their frame, their scope and their arguments in the
// machine return what calls that allocate them return: a super that
// passes its arguments on to a method that keeps them in a lambda, a
// method with more variables than a scope holds itself, arguments read
// after calls deeper down reused the stack, a call left by an
// exception, and a call with the wrong number of arguments.
func TestCallsThatAllocateNothing(t *testing.T) {
	expectOutput(t, `class Base
  def keep(a, b)
    lambda { [a, b] }
  end
  def sum(a, b)
    a + b
  end
end
class Kid < Base
  def keep(a, b)
    super
  end
  def sum(a, b)
    x = super
    x + super(a, 1)
  end
end
def wide(a, b, c)
  d = a + b
  e = d + c
  f = e + 1
  g = a == 0 ? 0 : wide(a - 1, b, c)
  [d, e, f].sum + g
end
def args_survive(a, b)
  x = second(b, a)
  y = second(a + 10, b + 10)
  [a, b, x, y]
end
def second(a, b)
  third(a * 2, b * 2) + a
end
def third(a, b)
  a - b
end
def fails(n)
  raise ArgumentError, "at #{n}" if n == 0
  fails(n - 1)
end
kid = Kid.new
kept = kid.keep(1, 2)
kid.keep(3, 4)
wide(5, 6, 7)
p kept.call, kid.sum(2, 3), wide(3, 1, 2), args_survive(1, 2)
begin
  fails(3)
rescue => e
  p e.message
end
begin
  third(1)
rescue ArgumentError => e
  p e.message
end
p third(5, 3), (1..3).map { |i| third(i, 1) }
`, `[1, 2]
8
50
[1, 2, 4, 9]
"at 0"
"wrong number of arguments (given 1, expected 2)"
2
[0, 1, 2]
`)
}

// A proc whose code writes no block or lambda and starts no async call
// is contained; any other keeps its frame.
func TestContainedProcs(t *testing.T) {
	prog, err := syntax.Parse("t.cameo", "def plain(a)\n  b = a + 1\n  begin\n    b.to_s\n  rescue => e\n    nil\n  end\nend\ndef blocky(a)\n  a.map { |x| x }\nend\ndef lam(a)\n  -> { a }\nend\ndef deflt(a = [1].map { |x| x })\n  a\nend\n")
	assert.NoError(t, err)
	unit, err := ir.Compile("t.cameo", prog)
	assert.NoError(t, err)
	in := New(nil, nil)
	got := map[string]bool{}
	for i, p := range unit.Procs {
		got[p.GetName()] = in.paramsOf(procOf(unit, uint32(i))).contained
	}
	assert.True(t, got["plain"] && got["block"])
	assert.True(t, !got["blocky"] && !got["lam"] && !got["deflt"])
}
