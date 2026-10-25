package cameo

import (
	"bytes"
	"strings"
	"testing"
)

// TestStdoutAndStderrAreStreams proves $stdout and $stderr name STDOUT and
// STDERR, whose puts, print and write go to the script's two streams, as
// warn does, and that assigning them or any other global is refused.
func TestStdoutAndStderrAreStreams(t *testing.T) {
	var out, errOut bytes.Buffer
	src := "puts \"a\"\n$stderr.puts \"b\", [1, 2]\nSTDOUT.print \"c\", 1, \"\\n\"\nn = $stderr.write(\"e\\n\")\nputs n, $stdout.sync, STDERR.fileno, $stdout.flush == STDOUT\nwarn \"w\"\n"
	prog, err := Load("t.cameo", src, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "a\nc1\n2\ntrue\n2\ntrue\n" {
		t.Fatalf("stdout %q", out.String())
	}
	if errOut.String() != "b\n1\n2\ne\nw\n" {
		t.Fatalf("stderr %q", errOut.String())
	}
	for src, want := range map[string]string{
		"$stdout = nil\n": "$stdout cannot be reassigned: it is the script's own stream",
		"$x = 1\n":        "global variables are not supported",
		"p $x\n":          "global variables are not supported",
	} {
		_, err := Load("t.cameo", src, &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v; want %q", src, err, want)
		}
	}
}
