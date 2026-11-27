# getoptlong: testcase to demonstrate that Getopt::Options declares switches
# in a class body
#
# The expected values are Ruby 3.4's, with getoptlong 0.2.1. Ruby takes the
# options out of ARGV; cameo leaves ARGV and gives the operands in rest.
# Parsing the script's own arguments is checked in tests/getoptlong_test.go.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "getoptlong"
class Options < Getopt::Options
  bool :verbose, default: false
  string :name, default: "world"
  int :count, default: 1
  string :level, default: "none"

  on("--verbose", "-v") { |o, _| o.with(verbose: true) }
  on("--name", "-n", arg: true) { |o, arg| o.with(name: arg) }
  on("--count", "-c", arg: true) { |o, arg| o.with(count: Integer(arg)) }
  on("--level", "-l", arg: :optional) { |o, arg| o.with(level: arg.empty? ? "some" : arg) }
end

options, files = Options.parse(%w[-v --name=Ann -c 2 a.txt -l --level=high -- -x])
p options, files
p Options.switches.map(&:names), Getopt::Options.switches
begin
  Options.parse(%w[--bogus], quiet: true)
rescue GetoptLong::InvalidOption => e
  puts e.message
end

__END__
Options(verbose: true, name: "Ann", count: 2, level: "high")
["a.txt", "-x"]
[["--verbose", "-v"], ["--name", "-n"], ["--count", "-c"], ["--level", "-l"]]
[]
unrecognized option `--bogus'
