package eval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/syntax"
)

// run evaluates a script and returns its standard output and the error that
// ended it.
func run(t *testing.T, src string) (string, error) {
	t.Helper()
	prog, err := syntax.Parse("t.arb", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out bytes.Buffer
	in := New(&out, &out)
	err = in.Run("t.arb", prog)
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
	expectError(t, "x = 1\nx.nope\n", "t.arb:2: undefined method 'nope' for an instance of Integer (NoMethodError)")
	expectError(t, "nil.foo\n", "undefined method 'foo' for nil (NoMethodError)")
	expectError(t, "def f(a); end\nf\n", "t.arb:2: wrong number of arguments (given 0, expected 1) (ArgumentError)")
	expectError(t, "def f(a, b = 1); end\nf(1, 2, 3)\n", "(given 3, expected 1..2)")
	expectError(t, "Nope\n", "uninitialized constant Nope (NameError)")
	expectError(t, "1 / 0\n", "divided by 0 (ZeroDivisionError)")
	expectError(t, "raise \"custom\"\n", "t.arb:1: custom (RuntimeError)")
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
	expectError(t, "class A\n def x; end\n def x; end\nend\n", "method x is already defined in A (AmberError)")
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

func TestTimeCrossesAsync(t *testing.T) {
	// A time travels to a process and back with its instant and offset.
	src := `
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
