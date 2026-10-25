package cameo

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/protogen"
)

// pausing loads src with a host method Test.pause that asks the script to
// pause, so the pause lands before the statement after the call.
func pausing(t *testing.T, src string) (*Program, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	prog, err := Load("t.cameo", src, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	prog.Module("Test").Def("pause", func([]any, map[string]any) (any, error) {
		prog.Pause()
		return nil, nil
	})
	return prog, &out
}

const pausingScript = `require "set"
class Point
  int :x, :y
  def boom = raise(ArgumentError, "boom")
end
include Cameo
LIMIT = 3
Alias = Point
a = Point.new(x: 1, y: 2)
f = ->(x) { x }
job = async a, :x
failed = async a, :boom
puts "one"
Test.pause
puts "two"
puts a.x + LIMIT + await(job) + Alias.new(x: 5, y: 0).x
begin
  await failed
rescue ArgumentError => e
  puts "rescued #{e.message}"
end
puts Set[1, 2].size
`

// TestPauseDumpsAndResumeContinues proves a paused script's dump resumes
// where it paused: the statements before the pause do not run again, the
// variables, constants and settled promises come back, and a value the
// envelope cannot carry is reported missing.
func TestPauseDumpsAndResumeContinues(t *testing.T) {
	prog, out := pausing(t, pausingScript)
	err := prog.Run()
	if !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if out.String() != "one\n" {
		t.Fatalf("output before the pause: %q", out.String())
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if dump.Line != 15 {
		t.Fatalf("paused before line %d, want 15", dump.Line)
	}
	if got, want := strings.Join(dump.Missing, ";"), "f (a Proc cannot be serialised)"; got != want {
		t.Fatalf("missing %q, want %q", got, want)
	}
	var resumed bytes.Buffer
	cont, err := Resume(dump.Data, &resumed, &resumed)
	if err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := resumed.String(), "two\n10\nrescued boom\n2\n"; got != want {
		t.Fatalf("output after the resume %q, want %q", got, want)
	}
}

// TestInspectDumpShowsTheState proves InspectDump names the pause point
// and every binding and promise as p would print them.
func TestInspectDumpShowsTheState(t *testing.T) {
	prog, _ := pausing(t, pausingScript)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	want := `script: t.cameo
paused: before line 15
variables:
  a = Point(x: 1, y: 2)
  f = (a Proc cannot be serialised)
  job = 1
  failed = 2
constants:
  Alias = Point
  LIMIT = 3
promises:
  1 x = 1
  2 boom raised boom (ArgumentError)
`
	if text != want {
		t.Fatalf("inspect:\n%s\nwant:\n%s", text, want)
	}
}

// TestResumedRandomNumbersContinue proves the random number generator
// resumes where it paused: a seeded script draws the same numbers paused
// and resumed as in one run, and keeps its seed.
func TestResumedRandomNumbersContinue(t *testing.T) {
	const src = "srand(-7)\na = rand(1000)\nTest.pause\nputs a, rand(1000), rand(1000), Random.seed\n"
	whole, out := pausing(t, strings.Replace(src, "Test.pause\n", "", 1))
	if err := whole.Run(); err != nil {
		t.Fatal(err)
	}
	prog, _ := pausing(t, src)
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	var resumed bytes.Buffer
	cont, err := Resume(dump.Data, &resumed, &resumed)
	if err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if resumed.String() != out.String() {
		t.Fatalf("resumed run printed %q, one run %q", resumed.String(), out.String())
	}
}

// TestPauseAfterTheLastStatementEndsTheRun proves a pause asked for
// during the last statement does not pause: there is nothing left to
// continue, and Run ends as it would have.
func TestPauseAfterTheLastStatementEndsTheRun(t *testing.T) {
	prog, out := pausing(t, "puts 1\nTest.pause\n")
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatalf("output %q", out.String())
	}
	if _, err := prog.Dump(); err == nil {
		t.Fatal("Dump of a script that did not pause returned no error")
	}
}

// TestResumeRejectsWhatIsNotADump proves Resume and InspectDump fail on
// bytes that are not a dump, rather than resuming an empty script.
func TestResumeRejectsWhatIsNotADump(t *testing.T) {
	if _, err := Resume([]byte("puts 1\n"), nil, nil); err == nil {
		t.Fatal("Resume of a script's text returned no error")
	}
	if _, err := InspectDump([]byte{0xff}); err == nil {
		t.Fatal("InspectDump of garbage returned no error")
	}
}

// TestResumeNamesARandomStateItCannotRead proves a dump whose generator
// state is not MT19937's, a cameo's before Random followed CRuby's, fails
// to resume with an error that says so.
func TestResumeNamesARandomStateItCannotRead(t *testing.T) {
	prog, _ := pausing(t, "rand\nTest.pause\nputs 1\n")
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	d, err := protogen.UnmarshalDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	d.Rand = make([]byte, 20)
	data, err := protogen.MarshalDump(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resume(data, nil, nil); err == nil || !strings.Contains(err.Error(), "random number state") {
		t.Fatalf("Resume returned %v, want an error about the random number state", err)
	}
}
