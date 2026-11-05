package tests

import (
	"bytes"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// The expected values are Ruby 3.4's, with optparse 0.6.0, but where a
// comment says otherwise.

// runArgs runs a script with arguments, as cameo script args... does.
func runArgs(t *testing.T, src string, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", src, &stdout, &stderr)
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
	assert.Equals(t, runScript(t, `p ARGV`), "[]\n")
}

// The parse! and order! lines differ from Ruby: an Array is a value, so
// argv keeps its options and order! sees them again.
func TestOptionParserParsesAsRubysDoes(t *testing.T) {
	out := runScript(t, `require "optparse"
o = OptionParser.new do |op|
  op.banner = "Usage: tool [options] FILES"
  op.separator ""
  op.separator "Specific options:"
  op.on("-v", "--[no-]verbose", "Run verbosely")
  op.on("-n", "--name NAME", String, "Your name")
  op.on("-c", "--count N", Integer, "How many")
  op.on("-r", "--rate R", Float, "A rate")
  op.on("-l", "--list x,y,z", Array, "A list")
  op.on("-o", "--optional [ARG]", "Optional arg")
  op.on("--level LEVEL", [:low, :high], "Select level (low, high)")
  op.on("-x", "Short only")
  op.on("--long-only", "Long only")
  op.on_tail("-h", "--help", "Show help")
  op.on_tail("--version", "Show version")
end
puts o.help
p o.banner, o.to_s == o.help, o.summary_width, o.summary_indent, o.program_name, OptionParser::Version
p o.parse(%w[-v -n Ann -c 3 -r 2.5 -l a,b,c file1 --level high -x --long-only file2]), o.options
p o.parse(%w[--no-verbose --name=Bob -nCarl -o -ofoo --optional=bar --optional baz]), o.options
p o.parse(%w[--verb --na Dee file -- -x --count 1]), o.options
argv = %w[-v f1 -c 2 f2]
p o.parse!(argv), argv, o.options
p o.order!(argv), argv, o.options
p o.permute(%w[a -n x b]), o.options, o.order(%w[a -n x b]), o.options
[%w[--bogus], %w[-z], %w[--name], %w[-c foo], %w[--level mid], %w[--verbose=yes], %w[-r x], %w[-c=7], %w[-n]].each do |args|
  begin
    o.parse(args)
  rescue OptionParser::ParseError => e
    p [e.class, e.message, e.args, e.reason]
  end
end
begin
  OptionParser.new { |op| op.on("--alpha") {}; op.on("--alps") {} }.parse(%w[--al])
rescue OptionParser::AmbiguousOption => e
  puts e.message
end
p OptionParser::InvalidOption.superclass, OptionParser::ParseError.superclass, OptionParser::MissingArgument.superclass, OptionParser::InvalidArgument.superclass, OptionParser::AmbiguousOption.superclass, OptionParser::NeedlessArgument.superclass
o = OptionParser.new
o.on("-a", "--all", "All of it")
o.on("--size SIZE", Integer, "Size", "in bytes")
o.on("-q", "--quiet")
o.banner = "Usage: x"
o.version = "1.2.3"
o.program_name = "prog"
p o.help, o.ver, o.summarize, o.to_a
o = OptionParser.new("Usage: given [opts]", 20, "  ")
o.on("-a", "--all", "All")
p o.help
o = OptionParser.new do |op|
  op.on("-f", "--flag") { |v| p v }
  op.on("--no-thing") { |v| p v }
  op.on("-i", "--int N", Integer) { |v| p v }
  op.on("-F", "--float F", Float) { |v| p v }
  op.on("-b", "--bool B", TrueClass) { |v| p v }
  op.on("-n", "--num N", Numeric) { |v| p v }
  op.on("-t", "--type TYPE", %w[fast slow]) { |v| p v }
  op.on("-m", "--mode MODE", {"a" => 1, "b" => 2}) { |v| p v }
end
p o.parse(%w[-f --no-thing -i 0x10 -i 1_000 -i 010 -F 1e3 -b yes -b no -b false -n 3 -n 2.5 -t fa -m b x]), o.options
begin
  o.parse(%w[-t medium])
rescue OptionParser::InvalidArgument => e
  puts e.message
end
begin
  OptionParser.new { |op| op.on("-s", "--sym SYM", Symbol) {} }
rescue ArgumentError => e
  puts e.message
end
o = OptionParser.new
o.on("-v") { }
o.on("--help") { }
p o.parse(%w[--help]), o.parse([]), o.parse(%w[a b]), o.parse(%w[-- -v]), o.parse(%w[-vv]), o.parse(%w[-v -v])
o = OptionParser.new { |op| op.on("-abc") { |v| p v }; op.on("-d", "--delay[=N]", Integer) { |v| p v } }
p o.parse(%w[-a -d -d5 --delay=7 --delay]), o.options
o = OptionParser.new
o.on("-x", "--example", "desc")
o.on("--very-long-option-name-that-is-wide VALUE", "wide desc")
o.separator "end"
puts o.help
begin
  OptionParser.new { |op| op.on("-v", "--verbose") {} }.parse(%w[--no-verbose])
rescue OptionParser::InvalidOption => e
  puts e.message
end
p OptionParser.new { |op| op.on("-n", "--name NAME") {} }.parse(%w[--name --other])
p OptionParser.new { |op| op.on("-n", "--name NAME") {}; op.on("-o", "--other") {} }.parse(%w[--name --other])
p OptionParser.new { |op| op.on("-n", "--name NAME") {} }.getopts(%w[-n x -b y], "n:b", "long:", "flag")
p OptionParser.new.getopts(%w[--long=1 --flag r], "ab:", "long:", "flag")
p ARGV
`)
	assert.Equals(t, out, `Usage: tool [options] FILES

Specific options:
    -v, --[no-]verbose               Run verbosely
    -n, --name NAME                  Your name
    -c, --count N                    How many
    -r, --rate R                     A rate
    -l, --list x,y,z                 A list
    -o, --optional [ARG]             Optional arg
        --level LEVEL                Select level (low, high)
    -x                               Short only
        --long-only                  Long only
    -h, --help                       Show help
        --version                    Show version
"Usage: tool [options] FILES"
true
32
"    "
"t"
"0.6.0"
["file1", "file2"]
{verbose: true, name: "Ann", count: 3, rate: 2.5, list: ["a", "b", "c"], level: :high, x: true, :"long-only" => true}
[]
{verbose: false, name: "Carl", optional: "baz"}
["file", "-x", "--count", "1"]
{verbose: true, name: "Dee"}
["f1", "f2"]
["-v", "f1", "-c", "2", "f2"]
{verbose: true, count: 2}
["f1", "-c", "2", "f2"]
["-v", "f1", "-c", "2", "f2"]
{verbose: true}
["a", "b"]
{name: "x"}
["a", "-n", "x", "b"]
{}
[OptionParser::InvalidOption, "invalid option: --bogus", ["--bogus"], "invalid option"]
[OptionParser::InvalidOption, "invalid option: -z", ["-z"], "invalid option"]
[OptionParser::MissingArgument, "missing argument: --name", ["--name"], "missing argument"]
[OptionParser::InvalidArgument, "invalid argument: -c foo", ["-c", "foo"], "invalid argument"]
[OptionParser::InvalidArgument, "invalid argument: --level mid", ["--level", "mid"], "invalid argument"]
[OptionParser::NeedlessArgument, "needless argument: --verbose=yes", ["--verbose=yes"], "needless argument"]
[OptionParser::InvalidArgument, "invalid argument: -r x", ["-r", "x"], "invalid argument"]
[OptionParser::InvalidArgument, "invalid argument: -c=7", ["-c=7"], "invalid argument"]
[OptionParser::MissingArgument, "missing argument: -n", ["-n"], "missing argument"]
ambiguous option: --al
OptionParser::ParseError
RuntimeError
OptionParser::ParseError
OptionParser::ParseError
OptionParser::ParseError
OptionParser::ParseError
"Usage: x\n    -a, --all                        All of it\n        --size SIZE                  Size\n                                     in bytes\n    -q, --quiet\n"
"prog 1.2.3"
["    -a, --all                        All of it\n", "        --size SIZE                  Size\n", "                                     in bytes\n", "    -q, --quiet\n"]
["Usage: x", "    -a, --all                        All of it\n", "        --size SIZE                  Size\n", "                                     in bytes\n", "    -q, --quiet\n"]
"Usage: given [opts]\n  -a, --all            All\n"
true
false
16
1000
8
1000.0
true
false
false
3
2.5
"fast"
2
["x"]
{flag: true, thing: false, int: 8, float: 1000.0, bool: false, num: 2.5, type: "fast", mode: 2}
invalid argument: -t medium
unsupported argument type: Symbol
[]
[]
["a", "b"]
["-v"]
[]
[]
"-d"
5
7
nil
[]
{a: "-d", delay: nil}
Usage: t [options]
    -x, --example                    desc
        --very-long-option-name-that-is-wide VALUE
                                     wide desc
end
invalid option: --no-verbose
[]
[]
{"n" => "x", "b" => true, "long" => nil, "flag" => false}
{"a" => false, "b" => nil, "long" => "1", "flag" => true}
[]
`)
}

func TestOptionParserParsesARGVByDefault(t *testing.T) {
	out, _ := runArgs(t, `require "optparse"
o = OptionParser.new { |o| o.on("-n", "--name NAME") }
rest = o.parse!
p rest, o.options[:name], ARGV
p OptionParser.new.getopts(%w[-v --name=Bob x], "v", "name:")`, "-n", "Ann", "file", "--name=Bob")
	assert.Equals(t, out, "[\"file\"]\n\"Bob\"\n[\"-n\", \"Ann\", \"file\", \"--name=Bob\"]\n{\"v\" => true, \"name\" => \"Bob\"}\n")
}

func TestOptionParserRecordsWhatItParsedInOptions(t *testing.T) {
	out := runScript(t, `require "optparse"
o = OptionParser.new do |op|
  op.on("-v", "--[no-]verbose")
  op.on("-n", "--name NAME")
  op.on("-c", "--count N", Integer)
  op.on("-x")
  op.on("--level LEVEL", [:low, :high]) { |v| puts "level #{v}" }
end
p o.options
p o.parse(%w[-v --name=Ann -c 3 -x --level high file]), o.options
p o.parse(%w[--no-verbose]), o.options
p o.options[:name], o.options.fetch(:count, 1)
`)
	assert.Equals(t, out, `{}
level high
["file"]
{verbose: true, name: "Ann", count: 3, x: true, level: :high}
[]
{verbose: false}
nil
1
`)
}

func TestOptionParserRefusesInto(t *testing.T) {
	out := runScript(t, `require "optparse"
begin
  OptionParser.new { |o| o.on("-v") }.parse(%w[-v], into: {})
rescue ArgumentError => e
  puts e.message
end`)
	assert.Equals(t, out, "into: fills a Hash in place, which a value cannot be; read options after the parse, or use getopts, which returns the Hash\n")
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
