package syntax

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parse(t *testing.T, src string) *Program {
	t.Helper()
	prog, err := Parse("test.cameo", src)
	if err != nil {
		t.Fatalf("parse failed:\n%v", err)
	}
	return prog
}

func rejects(t *testing.T, src, want string) {
	t.Helper()
	_, err := Parse("test.cameo", src)
	if err == nil {
		t.Fatalf("accepted %q, want an error containing %q", src, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error for %q is\n  %v\nwant it to contain %q", src, err, want)
	}
}

// TestExamplesPass holds every example script to the checker: the corpus is
// the specification, so every script in it is a valid cameo program.
func TestExamplesPass(t *testing.T) {
	scripts, err := filepath.Glob("../../examples/*.cameo")
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, script := range scripts {
		src, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(filepath.Base(script), string(src)); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestChainedAssignmentRewrites(t *testing.T) {
	prog := parse(t, "a = nil\na.x.y = 3\n")
	assign, ok := prog.Body[1].(*Assign)
	if !ok || assign.Name != "a" {
		t.Fatalf("want a rebinding of a, got %#v", prog.Body[1])
	}
	outer, ok := assign.Value.(*With)
	if !ok {
		t.Fatalf("want With, got %#v", assign.Value)
	}
	if v, ok := outer.Target.(*Var); !ok || v.Name != "a" {
		t.Fatalf("outer target: %#v", outer.Target)
	}
	if k, ok := outer.Key.(*Sym); !ok || k.Name != "x" {
		t.Fatalf("outer key: %#v", outer.Key)
	}
	inner, ok := outer.Value.(*With)
	if !ok {
		t.Fatalf("want nested With, got %#v", outer.Value)
	}
	if call, ok := inner.Target.(*Call); !ok || call.Name != "x" {
		t.Fatalf("inner target reads a.x: %#v", inner.Target)
	}
	if k, ok := inner.Key.(*Sym); !ok || k.Name != "y" {
		t.Fatalf("inner key: %#v", inner.Key)
	}
	if v, ok := inner.Value.(*Int); !ok || v.Value != 3 {
		t.Fatalf("inner value: %#v", inner.Value)
	}
	if assign.Line() != 2 {
		t.Fatalf("line %d, want 2", assign.Line())
	}
}

func TestIndexAssignmentRewrites(t *testing.T) {
	prog := parse(t, "a = nil\na.tags[0] = \"uno\"\n")
	assign := prog.Body[1].(*Assign)
	outer := assign.Value.(*With)
	inner := outer.Value.(*With)
	if k, ok := inner.Key.(*Int); !ok || k.Value != 0 {
		t.Fatalf("index key: %#v", inner.Key)
	}
	if call, ok := inner.Target.(*Call); !ok || call.Name != "tags" {
		t.Fatalf("inner target: %#v", inner.Target)
	}
	prog = parse(t, "h = {}\nh[:k] = 1\nh[:n] += 1\n")
	if _, ok := prog.Body[1].(*Assign); !ok {
		t.Fatalf("index assignment: %#v", prog.Body[1])
	}
	with := prog.Body[2].(*Assign).Value.(*With)
	if _, ok := with.Value.(*Call); !ok {
		t.Fatalf("h[:n] += 1 carries the sum: %#v", with.Value)
	}
}

func TestFieldAssignmentInsideInitializeOnly(t *testing.T) {
	parse(t, "class P\n int :x\n def initialize(x)\n  self.x = x\n  self.x.y = 1\n  self.x += 1\n end\nend\n")
	rejects(t, "class P\n def set(x)\n  self.x = x\n end\nend\n", "test.cameo:3: self.x = …: a field is set in initialize only")
	rejects(t, "self.x = 1\n", "a field is set in initialize only")
	rejects(t, "class P\n def initialize\n  self[:x] = 1\n end\nend\n", "self[…] = …: cameo has no setters")
}

func TestNoInstanceVariables(t *testing.T) {
	// @ is free: initialize sets a field with self.x =, and a field is read
	// with its reader.
	rejects(t, "class P\n int :x\n def initialize(x)\n  @x = x\n end\nend\n", "@x = …: cameo has no instance variables; initialize sets a field with self.x = …")
	rejects(t, "class P\n int :x\n def x2 = @x * 2\nend\n", "@x: cameo has no instance variables; a field is read with its reader, x or self.x")
	rejects(t, "class P\n int :x\n def initialize\n  @x.y = 1\n end\nend\n", "@x: cameo has no instance variables")
	rejects(t, "puts @x\n", "cameo has no instance variables")
}

func TestAssignmentNeedsALocalAtTheStart(t *testing.T) {
	rejects(t, "foo.x = 1\n", "the left side of an assignment must start with a local variable")
	rejects(t, "A.x = 1\n", "must start with a local variable")
	parse(t, "x = 1\nx = 2\n")
}

func TestHashNewBlockMayReadItsHash(t *testing.T) {
	// Only a store into the hash is rejected: reading it, or storing into
	// another local, is fine.
	parse(t, "h = Hash.new { |m, k| m.size + k }\n")
	parse(t, "h = Hash.new { |m, k| x = {}\nx[k] = 1\nx }\n")
}

func TestMutationRebindsALocal(t *testing.T) {
	// A mutating call on a local or a field in initialize parses; the
	// evaluator rebinds the local.
	parse(t, "a = []\na << 1 << 2\na.push(3)\na.map! { _1 }\n")
	parse(t, "class C\n  list :xs, of: :int\n  def initialize\n    self.xs = []\n    self.xs << 1\n    self.xs.push(2)\n  end\nend\n")
	rejects(t, "class C\n  list :xs, of: :int\n  def add(x) = self.xs << x\nend\n", "<< changes its receiver")
}

func TestBacktrackingRegexpIsRefused(t *testing.T) {
	// Go's regexp has no backtracking: a literal that needs it is refused
	// by the syntax check.
	rejects(t, "x = /(a)\\1/\n", "This regexp is not supported in Cameo: /(a)\\1/ uses a backreference")
}

func TestRegexpLiterals(t *testing.T) {
	// A regexp literal parses, with flags and with interpolation, whose
	// bare names may be locals.
	parse(t, "x = /a+b/i\ny = 1\nz = /#{y}c#{x}/m\n")
	parse(t, "case \"s\"\nwhen /s/ then 1\nend\n")
}

func TestForceEncodingOnATemporary(t *testing.T) {
	// force_encoding rebinds a local, and on anything else is a plain call
	// whose value is the converted string.
	parse(t, "s = \"a\".b\ns.force_encoding(\"UTF-8\")\nt = \"b\".b.force_encoding(\"UTF-8\")\n")
}

func TestBigIntLiterals(t *testing.T) {
	// An integer literal beyond 64 bits, negated or not, is a BigInt.
	parse(t, "x = 99999999999999999999\ny = -0xffffffffffffffff\n")
}

func TestSugarParses(t *testing.T) {
	// Forwarding, anonymous parameters, for loops, alias, a scoped class
	// name and a top-level def self.x are sugar the converter rewrites.
	parse(t, "def f(...) = g(...)\ndef h(*, **, &) = g(*, **, &)\n")
	parse(t, "for a, b in [[1, 2]]\nend\n")
	parse(t, "def a = 1\nalias b a\n")
	parse(t, "module M; end\nclass M::C; end\n")
	parse(t, "def self.top = 1\n")
	rejects(t, "alias :\"#{x}\" y\n", "computed name")
}

func TestRejections(t *testing.T) {
	cases := map[string]string{
		"attr_accessor :x\n":                 "attr_accessor is not supported",
		"def x=(v); end\n":                   "def x=: setters are not supported",
		"def save!; end\n":                   "bang methods are not supported",
		"A = []\nA << 1\n":                   "<< changes its receiver",
		"a = [[]]\na.first.push(1)\n":        "push changes its receiver",
		"A = {}\nA.merge!({})\n":             "merge!: a bang method changes its receiver",
		"h = Hash.new { |m, k| m[k] = 1 }\n": "Hash.new's block cannot store into its hash m",
		"$x = 1\n":                           "global variables are not supported",
		"@@x = 1\n":                          "class variables are not supported",
		"class << self; end\n":               "class << self is not supported",
		"x = `ls`\n":                         "backtick commands",
		"def m; def n; end; end\n":           "def n inside a method",
		"string :x\n":                        "which only a class body can do",
		"class P\n list :x\nend\n":           "needs of: the element type",
		"class P\n map :x, key: :float, value: :int\nend\n": "a key is a string, a bool or an integer type of 32 or 64 bits",
		"class P\n enum :k\nend\n":                          "needs values:",
		"class P\n reserved :x\nend\n":                      "the field number is a positive integer",
		"async foo\n":                                       "async takes a receiver, a method name",
		"await\n":                                           "await takes a process",
		"send(:x)\n":                                        "send is not supported",
		"private\ndef x; end\n":                             "private is not supported",
		"yield 1\n":                                         "yield outside a method",
		"def m; X = 1; end\n":                               "a constant cannot be assigned inside a method",
		"x = 1\nfoo(\n":                                     "parse error at line 3",
	}
	for src, want := range cases {
		rejects(t, src, want)
	}
}

func TestClassBodyDeclarations(t *testing.T) {
	prog := parse(t, `
class Person
  string :name, :nick, default: ""
  reserved 2, :old
  list :emails, of: :string
  map :scores, key: :string, value: :int
  ref :home, Address
  enum :role, values: [:admin, :user]
  include Describable
  def initialize(name); self.name = name; end
  def self.build = new("x")
end
`)
	cls := prog.Body[0].(*ClassDef)
	if cls.Name != "Person" || cls.Digest == "" {
		t.Fatalf("class: %#v", cls)
	}
	want := []string{"*syntax.FieldDecl", "*syntax.Reserved", "*syntax.FieldDecl", "*syntax.FieldDecl", "*syntax.FieldDecl", "*syntax.FieldDecl", "*syntax.Include", "*syntax.Def", "*syntax.Def"}
	for i, n := range cls.Body {
		if got := typeName(n); got != want[i] {
			t.Fatalf("body[%d] is %s, want %s", i, got, want[i])
		}
	}
	name := cls.Body[0].(*FieldDecl)
	if name.Type != "string" || len(name.Names) != 2 || name.Default == nil {
		t.Fatalf("string decl: %#v", name)
	}
	if r := cls.Body[1].(*Reserved); r.Number != 2 || r.Name != "old" {
		t.Fatalf("reserved: %#v", r)
	}
	if e := cls.Body[5].(*FieldDecl); e.Type != "enum" || len(e.Values) != 2 {
		t.Fatalf("enum: %#v", e)
	}
	if d := cls.Body[8].(*Def); !d.Singleton {
		t.Fatalf("def self.build is a singleton def")
	}
}

func TestDigestIgnoresLinesAndSeesBodies(t *testing.T) {
	a := parse(t, "class G\n def greet = 1\nend\n").Body[0].(*ClassDef)
	b := parse(t, "\n\nclass G\n\n def greet = 1\n\nend\n").Body[0].(*ClassDef)
	c := parse(t, "class G\n def greet = 2\nend\n").Body[0].(*ClassDef)
	if a.Digest != b.Digest {
		t.Fatalf("same body, different digests: %s %s", a.Digest, b.Digest)
	}
	if a.Digest == c.Digest {
		t.Fatalf("different bodies, same digest")
	}
}

func TestCallArguments(t *testing.T) {
	prog := parse(t, "f(1, *rest, k: 2, **o, &blk)\nf({a: 1})\n")
	call := prog.Body[0].(*Call)
	if len(call.Args) != 2 || len(call.KwArgs) != 2 || call.BlockArg == nil {
		t.Fatalf("call: args %d kwargs %d block %v", len(call.Args), len(call.KwArgs), call.BlockArg)
	}
	if _, ok := call.Args[1].(*Splat); !ok {
		t.Fatalf("second argument is a splat")
	}
	if call.KwArgs[1].Key != nil {
		t.Fatalf("**o is an entry without a key")
	}
	positional := prog.Body[1].(*Call)
	if len(positional.Args) != 1 || len(positional.KwArgs) != 0 {
		t.Fatalf("a braced hash is positional: %#v", positional)
	}
	if _, ok := positional.Args[0].(*Map); !ok {
		t.Fatalf("braced hash is a Map")
	}
}

func TestAsyncAndAwait(t *testing.T) {
	prog := parse(t, "j = async w, :run, 1, k: 2\nawait j\nawait w, :run, 1\nawait :all\n")
	a := prog.Body[0].(*Assign).Value.(*Async)
	if len(a.Args) != 1 || len(a.KwArgs) != 1 {
		t.Fatalf("async: %#v", a)
	}
	if m, ok := a.Method.(*Sym); !ok || m.Name != "run" {
		t.Fatalf("method: %#v", a.Method)
	}
	if w := prog.Body[1].(*Await); w.Process == nil {
		t.Fatalf("await process: %#v", w)
	}
	if w := prog.Body[2].(*Await); w.Process != nil || w.Recv == nil {
		t.Fatalf("await call form: %#v", w)
	}
	if _, ok := prog.Body[3].(*AwaitAll); !ok {
		t.Fatalf("await :all: %#v", prog.Body[3])
	}
}

func TestOperatorsAndControlFlow(t *testing.T) {
	prog := parse(t, "x = 1\ny = x + 1 if x && !x || x != 2\nz = -x\nw = x ||= 3\n")
	ifn := prog.Body[1].(*If)
	if _, ok := ifn.Cond.(*Or); !ok {
		t.Fatalf("cond: %#v", ifn.Cond)
	}
	if c, ok := prog.Body[2].(*Assign).Value.(*Call); !ok || c.Name != "-@" {
		t.Fatalf("unary minus: %#v", prog.Body[2])
	}
	if _, ok := prog.Body[3].(*Assign).Value.(*Assign).Value.(*Or); !ok {
		t.Fatalf("||= desugars to or: %#v", prog.Body[3])
	}
}

func typeName(n Node) string { return fmt.Sprintf("%T", n) }

func TestTapWithAHashPointsAtWith(t *testing.T) {
	rejects(t, "p = q.tap { |x| { a: 1 } }\n", "tap returns its receiver unchanged; for a copy with changes use with")
}
