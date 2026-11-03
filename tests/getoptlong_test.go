package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected values are Ruby 3.4's, with getoptlong 0.2.1. Ruby takes
// the options out of ARGV; cameo leaves ARGV and gives the operands in
// rest.

const getoptlongScript = `require "getoptlong"
opts = GetoptLong.new(["--help", "-h", GetoptLong::NO_ARGUMENT], ["--name", "-n", GetoptLong::REQUIRED_ARGUMENT], ["--level", "-l", GetoptLong::OPTIONAL_ARGUMENT], ["--verbose", GetoptLong::NO_ARGUMENT])
opts.quiet = ARGV[0] == "quiet"
opts.ordering = GetoptLong.const_get(ARGV[1])
seen = []
begin
  opts.each { |opt, arg| seen += [[opt, arg]] }
rescue GetoptLong::Error => e
  seen += [[e.class, e.message]]
end
p seen, opts.rest, opts.terminated?, opts.error?, opts.error_message, ARGV.size
`

func TestGetoptLongParsesAsRubysDoes(t *testing.T) {
	q := "`"
	for _, c := range []struct {
		args        []string
		out, stderr string
	}{
		{[]string{"loud", "PERMUTE", "-h", "--name", "Ann", "-l", "-lhigh", "--level=low", "--verbose", "file1", "file2"},
			`[["--help", ""], ["--name", "Ann"], ["--level", ""], ["--level", "high"], ["--level", "low"], ["--verbose", ""]]
["loud", "PERMUTE", "file1", "file2"]
true
nil
nil
11
`, ""},
		{[]string{"loud", "PERMUTE", "f1", "-n", "Bob", "f2", "--verb"}, `[["--name", "Bob"], ["--verbose", ""]]
["loud", "PERMUTE", "f1", "f2"]
true
nil
nil
7
`, ""},
		{[]string{"loud", "PERMUTE", "-h", "--", "-n", "x"}, `[["--help", ""]]
["loud", "PERMUTE", "-n", "x"]
true
nil
nil
6
`, ""},
		{[]string{"loud", "PERMUTE", "-hn", "Ann", "-l", "high", "-l", "-h"}, `[["--help", ""], ["--name", "Ann"], ["--level", "high"], ["--level", ""], ["--help", ""]]
["loud", "PERMUTE"]
true
nil
nil
8
`, ""},
		{[]string{"a", "REQUIRE_ORDER", "-h", "b"}, `[]
["a", "REQUIRE_ORDER", "-h", "b"]
true
nil
nil
4
`, ""},
		{[]string{"a", "RETURN_IN_ORDER", "-h", "b"}, `[["", "a"], ["", "RETURN_IN_ORDER"], ["--help", ""], ["", "b"]]
[]
true
nil
nil
4
`, ""},
		{[]string{"quiet", "PERMUTE", "--bogus"}, `[[GetoptLong::InvalidOption, "unrecognized option ` + q + `--bogus'"]]
["quiet", "PERMUTE"]
false
GetoptLong::InvalidOption
"unrecognized option ` + q + `--bogus'"
3
`, ""},
		{[]string{"quiet", "PERMUTE", "--name"}, `[[GetoptLong::MissingArgument, "option ` + q + `--name' requires an argument"]]
["quiet", "PERMUTE"]
false
GetoptLong::MissingArgument
"option ` + q + `--name' requires an argument"
3
`, ""},
		{[]string{"quiet", "PERMUTE", "--verbose=x"}, `[[GetoptLong::NeedlessArgument, "option ` + q + `--verbose' doesn't allow an argument"]]
["quiet", "PERMUTE"]
false
GetoptLong::NeedlessArgument
"option ` + q + `--verbose' doesn't allow an argument"
3
`, ""},
		{[]string{"loud", "PERMUTE", "-x"}, `[[GetoptLong::InvalidOption, "invalid option -- x"]]
["loud", "PERMUTE"]
false
GetoptLong::InvalidOption
"invalid option -- x"
3
`, "t.cameo: invalid option -- x\n"},
		{[]string{"loud", "PERMUTE", "-n"}, `[[GetoptLong::MissingArgument, "option requires an argument -- n"]]
["loud", "PERMUTE"]
false
GetoptLong::MissingArgument
"option requires an argument -- n"
3
`, "t.cameo: option requires an argument -- n\n"},
		{[]string{"loud", "PERMUTE", "--na", "Ann", "--l=x", "--v"}, `[["--name", "Ann"], ["--level", "x"], ["--verbose", ""]]
["loud", "PERMUTE"]
true
nil
nil
6
`, ""},
	} {
		out, stderr := runArgs(t, getoptlongScript, c.args...)
		assert.Equals(t, out, c.out)
		assert.Equals(t, stderr, c.stderr)
	}
}

func TestGetoptLongConstantsAndErrors(t *testing.T) {
	out := runScript(t, `require "getoptlong"
p GetoptLong::NO_ARGUMENT, GetoptLong::REQUIRED_ARGUMENT, GetoptLong::OPTIONAL_ARGUMENT, GetoptLong::PERMUTE, GetoptLong::REQUIRE_ORDER, GetoptLong::RETURN_IN_ORDER, GetoptLong::InvalidOption.superclass, GetoptLong::Error.superclass, GetoptLong::VERSION
[[["--help", GetoptLong::NO_ARGUMENT], ["--help", GetoptLong::NO_ARGUMENT]], [["help", GetoptLong::NO_ARGUMENT]], [["--help"]], [["--help", 7]]].each do |spec|
  begin
    GetoptLong.new(*spec)
  rescue ArgumentError => e
    puts e.message
  end
end
o = GetoptLong.new(["--help", "-h", GetoptLong::NO_ARGUMENT])
p o.get, o.terminated?, o.ordering, o.quiet?
begin
  o.ordering = GetoptLong::REQUIRE_ORDER
rescue RuntimeError => e
  puts e.message
end`)
	assert.Equals(t, out, "0\n1\n2\n1\n0\n2\nGetoptLong::Error\nStandardError\n\"0.2.1\"\noption redefined `--help'\nan invalid option `help'\nno argument-flag\nno argument-flag\nnil\ntrue\n1\nfalse\ninvoke ordering=, but option processing has already started\n")
}
