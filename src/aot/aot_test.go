package aot

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func build(t *testing.T, src string) *Result {
	t.Helper()
	res, err := Build("t.rb", src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", res.Go, 0); err != nil {
		t.Fatalf("the generated Go does not parse: %v\n%s", err, res.Go)
	}
	return res
}

func TestBuildCompilesIntegerMethods(t *testing.T) {
	res := build(t, `
def tak(x, y, z)
  unless y < x
    z
  else
    tak(tak(x - 1, y, z), tak(y - 1, z, x), tak(z - 1, x, y))
  end
end

def twice(n)
  tak(n, 0, 0) * 2
end

puts twice(3)
`)
	if got := strings.Join(res.Compiled, " "); got != "tak twice" {
		t.Errorf("compiled %q, want tak and twice", got)
	}
	for _, want := range []string{"func fn_tak(s *state, v_x int64, v_y int64, v_z int64) (int64, error)", "func big_tak(s *state, v_x *big.Int, v_y *big.Int, v_z *big.Int) (*big.Int, error)", "big_1 = big.NewInt(1)", "call_tak,", "func main()"} {
		if !strings.Contains(string(res.Go), want) {
			t.Errorf("the Go lacks %q:\n%s", want, res.Go)
		}
	}
}

// TestBuildLeavesTheRestToTheInterpreter checks the reason given for each
// method the compiler does not take.
func TestBuildLeavesTheRestToTheInterpreter(t *testing.T) {
	cases := []struct{ src, name, reason string }{
		{"def f(n)\n  puts n\n  n\nend", "f", "it calls puts, and it is not a method of the script"},
		{"def f(n)\n  n.to_s\nend", "f", "it calls to_s on Integer"},
		{"def f(n)\n  \"x\"\nend", "f", "a String is not compiled yet"},
		{"def f(n)\n  n < 1\nend", "f", "its result is true or false, not a number"},
		{"def f(n = 1)\n  n\nend", "f", "it takes optional, splat, keyword or block parameters"},
		{"def f(n)\n  n\nend\nclass A\n  def f(n)\n    n\n  end\nend", "f", "the program defines it more than once"},
		{"def f?(n)\n  n\nend", "f?", "its name is not a plain identifier"},
		{"def f(n)\n  if n > 0\n    1\n  end\nend", "f", "an if without an else is nil"},
		{"def f(n)\n  if n > 0\n    m = 1\n  end\n  n\nend", "f", "m is first set inside an if or a loop"},
		{"def f(n)\n  m = n\n  m = n > 1\n  1\nend", "f", "m holds Integer and true or false"},
		{"def f(n)\n  return\nend", "f", "return without a value"},
		{"def g(n)\n  \"#{n}\"\nend\ndef f(n)\n  g(n)\nend", "f", "it calls g, and it is left to the interpreter"},
		{"def f(n)\n  f(n, 1)\nend", "f", "it calls f with 2 arguments, not 1"},
		{"def f(n)\n  n.times { |i, j| i }\nend", "f", "the block of times takes more than one plain parameter"},
		{"def f(n)\n  n.each { |i| i }\nend", "f", "it calls each on Integer"},
		{"def f(n)\n  (1..n).each { |i| i }\nend", "f", "the value of each is a Range"},
		{"def f(n)\n  n.times { break 1 }\n  1\nend", "f", "break with a value is not compiled yet"},
		{"def f(n)\n  n.times { x = 1 }\n  x\nend", "f", "it calls x, and it is not a method of the script"},
		{"def f(n)\n  n.times { |i| if i > 1\n t = 1\n end }\n  1\nend", "f", "t is first set inside an if or a loop"},
		{"def f(n)\n  n.times { |i| puts i }\n  1\nend", "f", "it calls puts"},
		{"def f(n)\n  n.map { |i| i }\nend", "f", "the block of map is not compiled yet"},
		{"def f(n)\n  break\nend", "f", "break outside a loop or a block"},
		{"def f(n)\n  x = 1\n  x = 1.5\n  x\nend", "f", "x holds Integer and Float"},
		{"def f(n)\n  n > 0 ? 1 : 1.5\nend", "f", "the branches give Integer and Float"},
		{"def f(n)\n  return 1.5 if n > 0\n  1\nend", "f", "its value is Integer and it returns Float"},
		{"def f(n)\n  g(1.5)\nend\ndef g(x)\n  x\nend", "f", "an argument of g is Float, not Integer"},
		{"def f(n)\n  n.times { |i| i + 0.5 }\n  (n * 1.5).times { }\n  n\nend", "f", "a bound of times is Float, not Integer"},
		{"class A\n  int :n\n  def f = n\nend\nclass B\n  int :n\nend\ndef g(a)\n  a.n\nend", "g", "it calls n on Integer"},
		{"class A\n  string :s\n  def f = s\nend", "A#f", "a string field is not compiled yet"},
		{"class A\n  ref :b, B\n  def f = b\nend", "A#f", "the field b refers to B, which is not a top-level class of the script"},
		{"def f(n)\n  self\nend", "f", "self at the top level is not compiled"},
		{"class A\n  int :n\n  def f = g\n  def g = \"x\"\nend", "A#f", "it calls g, and it is left to the interpreter"},
		{"class A\n  int :n\n  def f(o) = o.n\nend\nclass B\n  float :n\nend\nclass C\n  def h = 1\nend\ndef k(c) = c.h + 1", "A#f", "it calls n on Integer"},
		{"class A\n  int :n\n  def initialize(n)\n    self.n = n\n    puts n\n  end\nend\ndef f(n) = A.new(n).n", "f", "A.new runs an initialize that does more than set fields"},
		{"class A\n  int :n\n  def initialize(n)\n    self.n = n\n  end\nend\ndef f(n) = A.new(n: n).n", "f", "A.new with keywords runs initialize, which takes positional arguments"},
		{"class A\n  int :n\nend\ndef f(n) = A.new(n).n", "f", "A.new takes its fields as keywords"},
		{"class A\n  int :n\nend\ndef f(n) = A.new(m: n).n", "f", "A.new: A has no field m"},
		{"class A\n  int :n\n  def initialize(n)\n    self.n = n\n  end\nend\ndef f(n) = A.new(n * 1.5).n", "f", "A.new takes Integer, not Float"},
		{"class A\n  float :x\n  def initialize(x)\n    self.x = x / 2\n  end\nend\ndef f(n) = A.new(1.5).x", "f", "A.new takes Integer, not Float"},
		{"def f(n) = B.new(n).n", "f", "it constructs B, which is not a top-level class of the script"},
		{"def f(n)\n  n.times { next }\n  n\nend\ndef g(n)\n  n.times { |i| return i }\n  n\nend\ndef h(n)\n  while n > 0\n    break\n    n = 1\n  end\n  n\nend", "h", "code follows a break"},
	}
	for _, c := range cases {
		res := build(t, c.src)
		reason, ok := res.Left[c.name]
		if !ok {
			t.Errorf("%q: %s compiled, want it left to the interpreter", c.src, c.name)
			continue
		}
		if !strings.Contains(reason, c.reason) {
			t.Errorf("%q: the reason is %q, want %q", c.src, reason, c.reason)
		}
	}
}

// TestBuildWritesBlockLoops holds the Go of times, each and downto to
// loops with the counter the loop's own, the stop check on the loop's
// variable, or on a counter where the variable is a big.Int, and break and
// next to Go's.
func TestBuildWritesBlockLoops(t *testing.T) {
	res := build(t, `
def f(n)
  s = 0
  n.times { |i| s += i }
  (1...n).each do |i|
    next if i == 2
    break if i == 5
    s += i
  end
  n.downto(1) { s += 1 }
  s
end
`)
	if got := strings.Join(res.Compiled, " "); got != "f" {
		t.Fatalf("compiled %q, want f", got)
	}
	for _, want := range []string{
		"for t2 := int64(0); t2 < t1; t2++ {", "if t2&4095 == 0 && eval.Stopped(s.ctx) {", "v_i := t2",
		"for t6 := t4; t6 < t5; t6++ {", "continue", "break",
		"if t9 == -1<<63 {", "for t10 := t8; t10 >= t9; t10-- {",
		"var t3 uint32", "for t2 := new(big.Int); t2.Cmp(t1) < 0; t2 = new(big.Int).Add(t2, big_1) {", "t3++", "if t3&4095 == 0 && eval.Stopped(s.ctx) {",
	} {
		if !strings.Contains(string(res.Go), want) {
			t.Errorf("the Go lacks %q:\n%s", want, res.Go)
		}
	}
}

// TestBuildTypesFloats holds that a method's Float result is found through
// its recursion and through the methods it calls, that mixed arithmetic
// converts the Integer, and that Integer against Float compares exactly.
func TestBuildTypesFloats(t *testing.T) {
	res := build(t, `
def f(n)
  return n.to_f if n > 3
  g(n + 1) * 2
end

def g(n)
  f(n)
end

def h(n)
  n % 2.5 < n && n == 2.5 ? n.to_i : n
end

def loops(n)
  loops(n)
end
`)
	if got := strings.Join(res.Compiled, " "); got != "f g h loops" {
		t.Fatalf("compiled %q, want f, g, h and loops", got)
	}
	for _, want := range []string{
		"func fn_f(s *state, v_n int64) (float64, error)", "func big_f(s *state, v_n *big.Int) (float64, error)",
		"func fn_g(s *state, v_n int64) (float64, error)", "func fn_h(s *state, v_n int64) (int64, error)",
		"func fn_loops(s *state, v_n int64) (int64, error)",
		"(t2 * float64(2))", "fmod(float64(v_n), 2.5)", "object.CompareNumbers(object.Int(v_n), object.Float(2.5))",
		"object.CompareNumbers(object.BigInt(v_n), object.Float(2.5))", "return object.Float(r), nil",
	} {
		if !strings.Contains(string(res.Go), want) {
			t.Errorf("the Go lacks %q:\n%s", want, res.Go)
		}
	}
}

// TestBuildCompilesClassMethods holds the Go of methods on classes with
// typed fields: the instance as self, a field read by its index with the
// parent's fields first, a parameter typed by the field read on it, and a
// direct call through a ref field.
func TestBuildCompilesClassMethods(t *testing.T) {
	res := build(t, `
class Vec
  float :x, :y
  def dot(other) = x * other.x + y * other.y
end

class Vec3 < Vec
  float :z
  def norm3 = dot(self) + z * z
end

class Pair
  ref :a, Vec
  int :weight
  def spin(n) = a.dot(a) * weight * n
end

def twice(v) = v.dot(v) * 2
`)
	if got := strings.Join(res.Compiled, " "); got != "Pair#spin Vec#dot Vec3#norm3 twice" {
		t.Fatalf("compiled %q, want Pair#spin, Vec#dot, Vec3#norm3 and twice", got)
	}
	for _, want := range []string{
		"func fn_Vec__dot(s *state, self *object.Instance, v_other *object.Instance) (float64, error)",
		"fieldFloat(self, 0)", "fieldFloat(v_other, 1)",
		"func fn_Vec3__norm3(s *state, self *object.Instance) (float64, error)", "fn_Vec__dot(s, self, self)", "fieldFloat(self, 2)",
		"fieldRef(self, 0)", "fieldInt(self, 1)", "fieldBig(self, 1)",
		"func fn_twice(s *state, v_v *object.Instance) (float64, error)",
		`classNamed(a0.Cls, "Vec")`, `"Vec#dot":`, "call_Vec__dot,", "self_, ok := self.AsInstance()",
	} {
		if !strings.Contains(string(res.Go), want) {
			t.Errorf("the Go lacks %q:\n%s", want, res.Go)
		}
	}
}

// TestBuildCompilesConstruction holds the Go of Class.new: the arguments
// read first, initialize's parameters as locals of a block, the fields by
// index with an Integer converted for a Float field, and eval.Construct
// with the class found through the state.
func TestBuildCompilesConstruction(t *testing.T) {
	res := build(t, `
class Vec
  float :x, :y
  def initialize(x, y)
    self.x = x
    self.y = y
  end
  def plus(other) = Vec.new(x + other.x, y + other.y)
end

class Tag
  int :n
  ref :v, Vec
end

def build(n) = Vec.new(n, 2)
def tag(n) = Tag.new(n: n, v: build(n)).n
`)
	if got := strings.Join(res.Compiled, " "); got != "Vec#plus build tag" {
		t.Fatalf("compiled %q, want Vec#plus, build and tag", got)
	}
	for _, want := range []string{
		"func fn_Vec__plus(s *state, self *object.Instance, v_other *object.Instance) (*object.Instance, error)",
		"func fn_build(s *state, v_n int64) (*object.Instance, error)",
		"v_x := float64(v_n)", "v_y := float64(2)", "[0] = object.Float(v_x)",
		`eval.Construct(s.ctx, s.class("Vec"), t`, `eval.Construct(s.ctx, s.class("Tag"), t`,
		"[0] = object.Int(v_n)", "[1] = object.InstanceValue(t", "return object.InstanceValue(r), nil",
		"classes map[string]*object.Class",
	} {
		if !strings.Contains(string(res.Go), want) {
			t.Errorf("the Go lacks %q:\n%s", want, res.Go)
		}
	}
}

func TestBuildWritesAPackageForAHost(t *testing.T) {
	res, err := Build("lib.rb", "def one\n  1\nend\n", Options{Package: "lib"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(res.Go)
	for _, want := range []string{"package lib", "const File = \"lib.rb\"", "const Script = `def one"} {
		if !strings.Contains(src, want) {
			t.Errorf("the Go lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, "func main()") {
		t.Errorf("a package for a host has a main:\n%s", src)
	}
}
