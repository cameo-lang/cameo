# optparse: testcase to demonstrate that option parser completes and formats
# as Ruby's does
#
# version= and banner= take any value; the help shows a placeholder as it was
# written; a prefix completes to the shortest candidate that prefixes the
# others; an unknown short option completes to a long one; --help with an
# argument is refused and --version without a version aborts; a --no-thing
# switch is recorded under :"no-thing", as into: records it; parse takes one
# String as the one argument. The answers are Ruby 3.4.1's.
#
# The expected values are Ruby 3.4's, with optparse 0.6.0, but where a comment
# says otherwise.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "optparse"
o = OptionParser.new; o.version = 1.0; p o.ver; o.banner = 42; puts o.help.lines.first
o = OptionParser.new; o.on("-d", "--delay[=N]", Integer); o.on("--name=NAME"); o.on("-nNAME"); o.on("-x", "--ex N"); o.on("-o [OPT]"); puts o.help
o = OptionParser.new; o.on("--list"); o.on("--list-all"); p o.parse(%w[--lis]); o.on("--size", "--sz"); p o.parse(%w[--s])
o = OptionParser.new; o.on("--name NAME") { |v| p v }; p o.parse(%w[-n Ann]); p o.parse(%w[-nBob])
begin; OptionParser.new.parse(%w[--help=x]); rescue OptionParser::NeedlessArgument => e; puts e.message; end
o = OptionParser.new; o.on("--[no-]color"); o.on("--no-thing"); o.parse(%w[--no-color --no-thing]); p o.options
o = OptionParser.new; o.on("-v"); p o.parse("-v"), o.parse("-v", "x")
begin; OptionParser.new.parse(%w[--version]); rescue SystemExit => e; p e.status; end

__END__
"optparse_option_parser_completes_and_formats_as_rubys_does__test.cameo 1.0"
42
Usage: optparse_option_parser_completes_and_formats_as_rubys_does__test.cameo [options]
    -d, --delay[=N]
        --name=NAME
    -nNAME
    -x, --ex N
    -o [OPT]
[]
[]
"Ann"
[]
"Bob"
[]
needless argument: --help=x
{color: false, "no-thing": false}
[]
["x"]
1
