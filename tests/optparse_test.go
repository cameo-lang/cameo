package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"github.com/cameo-lang/cameo/src/cameoio"
)

// The expected values are Ruby 3.4's, with optparse 0.6.0, but where a
// comment says otherwise.

// runArgs runs a script with arguments, as cameo script args... does.
func runArgs(t *testing.T, src string, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prog, err := cameo.LoadFromString("t.cameo", src, cameoio.Streams(&stdout, &stderr))
	if err != nil {
		t.Fatalf("%s\nfailed to load: %v", src, err)
	}
	prog.Args(args)
	if err := prog.Run(); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func TestARGVHoldsTheScriptsArguments(t *testing.T) {
	out, _ := runArgs(t, `p ARGV`, "-v", "file", "--", "x")
	assert.Equals(t, out, "[\"-v\", \"file\", \"--\", \"x\"]\n")
}

func TestOptionParserParsesARGVByDefault(t *testing.T) {
	out, _ := runArgs(t, `require "optparse"
o = OptionParser.new { |o| o.on("-n", "--name NAME") }
rest = o.parse!
p rest, o.options[:name], ARGV
p OptionParser.new.getopts(%w[-v --name=Bob x], "v", "name:")`, "-n", "Ann", "file", "--name=Bob")
	assert.Equals(t, out, "[\"file\"]\n\"Bob\"\n[\"-n\", \"Ann\", \"file\", \"--name=Bob\"]\n{\"v\" => true, \"name\" => \"Bob\"}\n")
}

func TestOptionParserHelpAndVersionExit(t *testing.T) {
	for _, c := range []struct{ args, out string }{
		{"--help", "Usage: t [options]\n    -v, --verbose                    Say more\n"},
		{"-h", "Usage: t [options]\n    -v, --verbose                    Say more\n"},
		{"--version", "t 1.2\n"},
	} {
		out, _ := runArgs(t, `require "optparse"
o = OptionParser.new
o.version = "1.2"
o.on("-v", "--verbose", "Say more")
begin
  o.parse!
rescue SystemExit => e
  puts "exit #{e.status}"
end`, c.args)
		assert.Equals(t, out, c.out+"exit 0\n")
	}
}
