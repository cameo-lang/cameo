package eval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/syntax"
)

// The tests in this file record where cameo deviates from Ruby on purpose:
// no value changes and no call rebinds its receiver, so a call of one of
// Ruby's mutators raises, and with IgnoreMutationErrors returns Ruby's
// result while the receiver keeps its value. See mutate.go.

// mutatorCases are Ruby's mutators of Array, String, Hash and Set: the
// receiver and how p shows it, the call on x, the method, what CRuby 3.4
// returns, and what x holds after the call in CRuby. In cameo x holds the
// receiver unchanged.
var mutatorCases = []struct{ recv, before, call, name, result, rubyAfter string }{
	{"[1, 2, 3]", "[1, 2, 3]", "x << 4", "<<", "[1, 2, 3, 4]", "[1, 2, 3, 4]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.push(4, 5)", "push", "[1, 2, 3, 4, 5]", "[1, 2, 3, 4, 5]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.unshift(0)", "unshift", "[0, 1, 2, 3]", "[0, 1, 2, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.insert(1, 9)", "insert", "[1, 9, 2, 3]", "[1, 9, 2, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.concat([4])", "concat", "[1, 2, 3, 4]", "[1, 2, 3, 4]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.pop", "pop", "3", "[1, 2]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.shift", "shift", "1", "[2, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.delete(2)", "delete", "2", "[1, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.delete_at(0)", "delete_at", "1", "[2, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.delete_if { _1 > 1 }", "delete_if", "[1]", "[1]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.keep_if { _1 > 1 }", "keep_if", "[2, 3]", "[2, 3]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.clear", "clear", "[]", "[]"},
	{"[1, 2, 3]", "[1, 2, 3]", "x.replace([7])", "replace", "[7]", "[7]"},
	{"\"ab\"", "\"ab\"", "x << \"c\"", "<<", "\"abc\"", "\"abc\""},
	{"\"ab\"", "\"ab\"", "x.concat(\"c\")", "concat", "\"abc\"", "\"abc\""},
	{"\"ab\"", "\"ab\"", "x.prepend(\"z\")", "prepend", "\"zab\"", "\"zab\""},
	{"\"ab\"", "\"ab\"", "x.insert(1, \"y\")", "insert", "\"ayb\"", "\"ayb\""},
	{"\"ab\"", "\"ab\"", "x.replace(\"q\")", "replace", "\"q\"", "\"q\""},
	{"\"ab\"", "\"ab\"", "x.clear", "clear", "\"\"", "\"\""},
	{"\"ab\"", "\"ab\"", "x.setbyte(0, 65)", "setbyte", "65", "\"Ab\""},
	{"\"ab\"", "\"ab\"", "x.bytesplice(0, 1, \"Z\")", "bytesplice", "\"Zb\"", "\"Zb\""},
	{"\"ab\"", "\"ab\"", "x.append_as_bytes(\"!\")", "append_as_bytes", "\"ab!\"", "\"ab!\""},
	{"{a: 1}", "{a: 1}", "x.store(:b, 2)", "store", "2", "{a: 1, b: 2}"},
	{"{a: 1}", "{a: 1}", "x.delete(:a)", "delete", "1", "{}"},
	{"{a: 1}", "{a: 1}", "x.shift", "shift", "[:a, 1]", "{}"},
	{"{a: 1}", "{a: 1}", "x.delete_if { true }", "delete_if", "{}", "{}"},
	{"{a: 1}", "{a: 1}", "x.keep_if { false }", "keep_if", "{}", "{}"},
	{"{a: 1}", "{a: 1}", "x.update(b: 2)", "update", "{a: 1, b: 2}", "{a: 1, b: 2}"},
	{"{a: 1}", "{a: 1}", "x.clear", "clear", "{}", "{}"},
	{"{a: 1}", "{a: 1}", "x.replace({c: 3})", "replace", "{c: 3}", "{c: 3}"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x << 3", "<<", "#<Set: {1, 2, 3}>", "#<Set: {1, 2, 3}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.add(3)", "add", "#<Set: {1, 2, 3}>", "#<Set: {1, 2, 3}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.add?(3)", "add?", "#<Set: {1, 2, 3}>", "#<Set: {1, 2, 3}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.delete(1)", "delete", "#<Set: {2}>", "#<Set: {2}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.delete?(1)", "delete?", "#<Set: {2}>", "#<Set: {2}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.merge([3])", "merge", "#<Set: {1, 2, 3}>", "#<Set: {1, 2, 3}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.subtract([1])", "subtract", "#<Set: {2}>", "#<Set: {2}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.delete_if { _1 > 1 }", "delete_if", "#<Set: {1}>", "#<Set: {1}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.keep_if { _1 > 1 }", "keep_if", "#<Set: {2}>", "#<Set: {2}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.clear", "clear", "#<Set: {}>", "#<Set: {}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.replace([9])", "replace", "#<Set: {9}>", "#<Set: {9}>"},
	{"Set[1, 2]", "#<Set: {1, 2}>", "x.reset", "reset", "#<Set: {1, 2}>", "#<Set: {1, 2}>"},
}

// runIgnoring is run with IgnoreMutationErrors set.
func runIgnoring(t *testing.T, src string) (string, error) {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out bytes.Buffer
	in := New(&out, &out)
	in.IgnoreMutationErrors = true
	if err := in.Run("t.cameo", prog); err != nil {
		return out.String(), &formatted{in.FormatError(err), err}
	}
	return out.String(), nil
}

func TestRubyMutatorsRaise(t *testing.T) {
	for _, c := range mutatorCases {
		expectError(t, "x = "+c.recv+"\n"+c.call+"\n", "cameo does not support mutating calls without rebinding")
	}
}

func TestIgnoredMutatorsReturnRubysResultAndLeaveTheReceiver(t *testing.T) {
	for _, c := range mutatorCases {
		src := "x = " + c.recv + "\np(" + c.call + ")\np x\n"
		got, err := runIgnoring(t, src)
		if err != nil {
			t.Fatalf("%s\nfailed: %v", src, err)
		}
		if want := c.result + "\n" + c.before + "\n"; got != want {
			t.Errorf("%s\ngot:\n%swant (Ruby leaves x as %s):\n%s", src, got, c.rubyAfter, want)
		}
	}
}

// TestMutatorsOfOtherReceiversAreCalls holds the check to the receiver's
// kind: Integer#<< shifts, and a class's own push is its own method.
func TestMutatorsOfOtherReceiversAreCalls(t *testing.T) {
	expectOutput(t, `
class Stack
  list :items, of: :int
  def push(x) = with(items: items + [x])
end
s = Stack.new(items: [])
p 1 << 3, s.push(1).items, s.items
`, "8\n[1]\n[]\n")
}

func TestFieldsAndSendRaiseForMutatorsToo(t *testing.T) {
	expectError(t, `
class C
  list :xs, of: :int
  def initialize
    self.xs = []
    self.xs << 1
  end
end
C.new
`, "cameo does not support mutating calls without rebinding; use a = a << 1, +, or append")
	expectError(t, "a = [1]\na.send(:push, 2)\n", "cameo does not support mutating calls without rebinding; use a = a.push(x), +, or append")
	expectError(t, "p [2, 1].send(:sort!)\n", "This is not supported, use x = x.sort instead")
	// A bang the checker lets through, since the script defines one of that
	// name, names its receiver at run time as the checker does.
	expectError(t, "class C\n  def sort! = 1\nend\nitems = [2, 1]\nitems.sort!\n", "This is not supported, use items = items.sort instead")
	got, err := runIgnoring(t, "p [2, 1].send(:sort!)\n")
	if err == nil || !strings.Contains(err.Error(), "This is not supported") {
		t.Fatalf("a bang method ran with mutation errors ignored: %q, %v", got, err)
	}
}

// TestAssigningAMutatorToItsReceiverRebinds: a mutator whose value is
// assigned back to its receiver returns Ruby's value, computed on a copy,
// so that the local or field ends up as Ruby's would; other locals bound
// to the old value keep it.
func TestAssigningAMutatorToItsReceiverRebinds(t *testing.T) {
	expectOutput(t, `
a = [2]
b = a
a = a << 1
p a, b
a <<= 5
p a
n = 2
n <<= 1
p n
s = "x"
s = s << "y"
p s
x = [1, 2]
x = x.pop
p x
class C
  list :xs, of: :int
  def initialize
    self.xs = []
    self.xs = self.xs << 1
  end
end
p C.new.xs
c = []
c = c << 1 << 2
c = c.push(3).push(4)
p c
`, "[2, 1]\n[2]\n[2, 1, 5]\n4\n\"xy\"\n2\n[1]\n[1, 2, 3, 4]\n")
	expectError(t, "a = [1]\nb = a << 2\n", "cameo does not support mutating calls without rebinding")
}

// TestAssignmentRebinds is the one rebinding cameo keeps: assignment to a
// field or an element makes a copy and rebinds the local, as with does.
func TestAssignmentRebinds(t *testing.T) {
	expectOutput(t, `
class P
  int :a
end
x = P.new(a: 1)
y = x
x.a = 2
h = {a: 1}
h[:b] = 2
p x.a, y.a, h
`, "2\n1\n{a: 1, b: 2}\n")
}

// TestForceEncodingReturnsACopy records the one mutator cameo keeps as a
// method: force_encoding returns the bytes relabelled and leaves the
// receiver, since nothing else reinterprets bytes. A call whose value the
// script drops raises, since in cameo it does nothing.
func TestForceEncodingReturnsACopy(t *testing.T) {
	expectOutput(t, `
b = "caf\xC3\xA9".b
u = b.force_encoding("UTF-8")
p u, u.encoding, b.encoding
`, "\"café\"\n#<Encoding:UTF-8>\n#<Encoding:BINARY (ASCII-8BIT)>\n")
	expectError(t, "s = \"ab\"\ns.force_encoding(\"BINARY\")\np s\n",
		"force_encoding returns a copy in cameo and changes nothing, and this call drops it; use x = x.force_encoding(...)")
	// The last statement of a body whose value is dropped drops its value
	// too: an if's branch, a loop's body, an ensure.
	expectError(t, "s = \"ab\"\nif true\n  s.force_encoding(\"BINARY\")\nend\np s\n", "this call drops it")
	expectError(t, "s = \"ab\"\nwhile s.size < 1\n  s.force_encoding(\"BINARY\")\nend\n[1].each { s.force_encoding(\"BINARY\") }\nwhile s.size < 3\n  s = s + \"c\"\n  s.force_encoding(\"BINARY\")\nend\n", "this call drops it")
	expectError(t, "s = \"ab\"\nbegin\n  1\nensure\n  s.force_encoding(\"BINARY\")\nend\np s\n", "this call drops it")
	// The script's last statement is the value LastValue reads, and an if
	// whose value is assigned keeps its branch's.
	expectOutput(t, "s = \"ab\"\nt = if true\n  s.force_encoding(\"BINARY\")\nend\np t.encoding\ns.force_encoding(\"BINARY\")\n", "#<Encoding:BINARY (ASCII-8BIT)>\n")
	got, err := runIgnoring(t, "s = \"ab\"\ns.force_encoding(\"BINARY\")\np s.encoding\n")
	if err != nil || got != "#<Encoding:UTF-8>\n" {
		t.Fatalf("with mutation errors ignored: %q, %v", got, err)
	}
}
