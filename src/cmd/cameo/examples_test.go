package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/syntax"
)

// examplesDir is the corpus of scripts that specify the language, relative
// to this package.
const examplesDir = "../../../examples"

// pending names the examples that need a milestone not built yet, with the
// milestone. A pending example must still fail: when it passes, it comes
// off this list.
var pending = map[string]string{}

// TestExamples runs every example script and holds its standard output to
// the expected file beside it. Pending examples are run too, and must fail.
func TestExamples(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.cameo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) == 0 {
		t.Fatalf("no example scripts under %s", examplesDir)
	}
	for _, script := range scripts {
		name := filepath.Base(script)
		t.Run(name, func(t *testing.T) {
			expected, err := os.ReadFile(strings.TrimSuffix(script, ".cameo") + ".expected")
			if err != nil {
				t.Fatalf("missing expected output: %v", err)
			}
			var stdout, stderr bytes.Buffer
			runErr := cameo.RunFile(script, &stdout, &stderr)
			if why, isPending := pending[name]; isPending {
				if runErr == nil && stdout.String() == string(expected) {
					t.Fatalf("passes, but is listed as pending (%s): take it off the list", why)
				}
				t.Skipf("pending: %s", why)
			}
			if runErr != nil {
				t.Fatalf("failed: %v\nstdout so far:\n%s", runErr, stdout.String())
			}
			if stdout.String() != string(expected) {
				t.Fatalf("output differs\n--- got ---\n%s--- want ---\n%s", stdout.String(), expected)
			}
		})
	}
}

// TestIRPrintsAListing proves cameo ir prints the instructions a script
// compiles to, and fails on a form the compiler does not compile yet,
// naming it and its line.
func TestIRPrintsAListing(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "double.cameo")
	if err := os.WriteFile(script, []byte("list = [1, 2]\nputs list.map { |x| x * 2 }.inspect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := irListing(script, &out); err != nil {
		t.Fatal(err)
	}
	want := `proc #0 <main>
  CONST 1
  CONST 2
  LIST 2 static
  STORE list
  DROP
  LOAD list
  SEND map 0 #1 list block
  SEND inspect 0
  SEND puts 1 self

proc #1 block(x)
  LOAD x
  CONST 2
  BINARY_TIMES
`
	if out.String() != want {
		t.Fatalf("listing differs\n--- got ---\n%s--- want ---\n%s", out.String(), want)
	}
	broken := filepath.Join(dir, "broken.cameo")
	if err := os.WriteFile(broken, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := run([]string{"ir", broken}); status != 1 {
		t.Fatalf("exit status %d for a script that does not parse, want 1", status)
	}
}

// TestExamplesCompileToTheIR proves every example compiles to a unit, all
// of it, and that the unit reads back from its wire form as it was
// written: no form of the language the corpus uses is left to the tree
// walker.
func TestExamplesCompileToTheIR(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.cameo"))
	if err != nil {
		t.Fatal(err)
	}
	compiled := 0
	for _, script := range scripts {
		src, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		prog, err := syntax.Parse(script, string(src))
		if err != nil {
			// An example that must fail the checker has no IR.
			continue
		}
		unit, err := ir.Compile(script, prog)
		if err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		data, err := proto.Marshal(unit)
		if err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		back := &cameopb.Unit{}
		if err := proto.Unmarshal(data, back); err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		if ir.Listing(back) != ir.Listing(unit) {
			t.Fatalf("%s: the unit reads back differently", script)
		}
		compiled++
	}
	if compiled == 0 {
		t.Fatalf("no example compiled under %s", examplesDir)
	}
}

// TestCheckReportsErrors proves the check command fails on a script the
// parser rejects and on one the checker rejects, so a green run on the
// corpus means something.
func TestCheckReportsErrors(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.cameo")
	if err := os.WriteFile(broken, []byte("def foo(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := check(broken); err == nil {
		t.Fatal("check accepted a script with a syntax error")
	}
	mutating := filepath.Join(dir, "mutating.cameo")
	if err := os.WriteFile(mutating, []byte("A = []\nA.sort!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := check(mutating)
	if err == nil || !strings.Contains(err.Error(), "mutating.cameo:2:") {
		t.Fatalf("check on a mutating script: %v", err)
	}
	if status := run([]string{"check", broken}); status != 1 {
		t.Fatalf("exit status %d for a syntax error, want 1", status)
	}
	if status := run([]string{broken}); status != 1 {
		t.Fatalf("exit status %d for running a broken script, want 1", status)
	}
}

// TestRunReportsExceptions proves an uncaught exception names its line and
// class and fails the run.
func TestRunReportsExceptions(t *testing.T) {
	script := filepath.Join(t.TempDir(), "boom.cameo")
	if err := os.WriteFile(script, []byte("puts 1\nraise ArgumentError, \"boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := cameo.RunFile(script, &stdout, &stderr)
	if err == nil {
		t.Fatal("an uncaught exception did not fail the run")
	}
	if !strings.Contains(err.Error(), "boom.cameo:2: boom (ArgumentError)") {
		t.Fatalf("error message: %v", err)
	}
	if stdout.String() != "1\n" {
		t.Fatalf("output before the exception: %q", stdout.String())
	}
}

// TestProtoCommand proves `cameo proto` prints one file per package, with
// the envelope, and writes them into a directory when given one.
func TestProtoCommand(t *testing.T) {
	var out bytes.Buffer
	if err := protoFiles(filepath.Join(examplesDir, "proto.cameo"), "", &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"// cameo.proto\n", "// cameo/value.proto\n", "// geo.proto\n", "message Person {", "message Address {", "message Point {", "message Value {"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, out.String())
		}
	}
	dir := t.TempDir()
	out.Reset()
	if err := protoFiles(filepath.Join(examplesDir, "proto.cameo"), dir, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "geo.proto")); err != nil {
		t.Fatalf("geo.proto not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cameo", "value.proto")); err != nil {
		t.Fatalf("cameo/value.proto not written: %v", err)
	}
}

// TestProtoCommandWritesAProcFieldAsAClosure proves `cameo proto` writes a
// class with a proc field, the field a closure of the envelope's file.
func TestProtoCommandWritesAProcFieldAsAClosure(t *testing.T) {
	script := filepath.Join(t.TempDir(), "routes.cameo")
	src := "class Route\n  string :path, number: 1\n  proc :handler, number: 2\nend\nclass App\n  list :routes, of: Route, number: 1\nend\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := protoFiles(script, "", &out); err != nil {
		t.Fatal(err)
	}
	want := `// cameo.proto
syntax = "proto3";

package cameo;

import "cameo/value.proto";

message App {
  repeated Route routes = 1;
}

message Route {
  optional string path = 1;
  optional cameo.Closure handler = 2;
}
`
	if !strings.HasPrefix(out.String(), want) {
		t.Fatalf("cameo proto wrote:\n%s\nwant it to start with:\n%s", out.String(), want)
	}
}

// TestProtoCommandWritesAnExceptionClass proves `cameo proto` writes a
// script's exception class as a message of its parts and its fields.
func TestProtoCommandWritesAnExceptionClass(t *testing.T) {
	script := filepath.Join(t.TempDir(), "errors.cameo")
	src := "class HTTPError < StandardError\n  int :status, number: 16\nend\n"
	if err := os.WriteFile(script, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := protoFiles(script, "", &out); err != nil {
		t.Fatal(err)
	}
	want := `// cameo.proto
syntax = "proto3";

package cameo;

import "cameo/value.proto";

message HTTPError {
  optional string message = 1;
  optional int64 line = 2;
  optional cameo.Value cause = 3;
  optional int64 status = 16;
}
`
	if !strings.HasPrefix(out.String(), want) {
		t.Fatalf("cameo proto wrote:\n%s\nwant it to start with:\n%s", out.String(), want)
	}
}
