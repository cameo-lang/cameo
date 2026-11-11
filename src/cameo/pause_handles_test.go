package cameo

import (
	"strings"
	"testing"
)

// A StringIO goes through a dump: with its string, where the script
// stands in it, and how it is open. It is one handle after the resume as
// it was before, for everything that held it: two variables, a list, the
// block of a builtin that is between two calls, a Proc.
func TestAStringIOGoesThroughADump(t *testing.T) {
	const src = `require "stringio"
io = StringIO.new("one\ntwo\nthree\n")
same = io
held = [io, StringIO.new("other")]
puts io.gets
Test.pause
puts same.gets, io.lineno, held[0].pos
out = StringIO.new
out.puts "written"
log = ->(line) { out.puts line }
Test.pause
log.("after the pause")
puts out.string
def drain(io)
  loop do
    line = io.gets
    break if line.nil?
    Test.pause
    puts "read #{line}"
  end
  io.eof?
end
puts drain(io), held[1].read, io.closed?
io.close
Test.pause
puts io.closed?, (io.gets rescue "closed")
`
	dump := sameAsWhole(t, src)
	everyPauseLands(t, src)
	if len(dump.Missing) != 0 {
		t.Fatalf("the dump left out %v", dump.Missing)
	}
	d := stackOf(t, dump)
	// The first dump: io, which three places hold, and the other one.
	if len(d.Handles) != 2 || d.Handles[0].Class != "StringIO" {
		t.Fatalf("the dump holds the handles %v", d.Handles)
	}
}

// A handle whose library has no way to save it is named when the run
// pauses, as any value the envelope cannot carry.
func TestAHandleWithoutAHookIsNamed(t *testing.T) {
	prog, _ := pausing(t, `require "strscan"
e = StringScanner.new("a b")
Test.pause
puts 1
`)
	if err := prog.Run(); err != ErrPaused {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(dump.Missing, ";"); !strings.Contains(got, "e (StringScanner is not a class with fields)") {
		t.Fatalf("missing: %s", got)
	}
}
