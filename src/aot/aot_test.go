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
		{"def f(n)\n  puts n\n  n\nend", "f", "it calls puts, and it is not a top-level method of the script"},
		{"def f(n)\n  n.to_s\nend", "f", "it calls to_s on Integer"},
		{"def f(n)\n  1.5\nend", "f", "a Float is not compiled yet"},
		{"def f(n)\n  n < 1\nend", "f", "its result is true or false, not an Integer"},
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
		{"def f(n)\n  n.times { x = 1 }\n  x\nend", "f", "it calls x, and it is not a top-level method"},
		{"def f(n)\n  n.times { |i| if i > 1\n t = 1\n end }\n  1\nend", "f", "t is first set inside an if or a loop"},
		{"def f(n)\n  n.times { |i| puts i }\n  1\nend", "f", "it calls puts"},
		{"def f(n)\n  n.map { |i| i }\nend", "f", "the block of map is not compiled yet"},
		{"def f(n)\n  break\nend", "f", "break outside a loop or a block"},
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
// loops with the counter the loop's own, and break and next to Go's.
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
		"for t2 := int64(0); t2 < t1; t2++ {", "v_i := t2",
		"for t6 := t4; t6 < t5; t6++ {", "continue", "break",
		"if t9 == -1<<63 {", "for t10 := t8; t10 >= t9; t10-- {",
		"for t2 := new(big.Int); t2.Cmp(t1) < 0; t2 = new(big.Int).Add(t2, big_1) {",
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
