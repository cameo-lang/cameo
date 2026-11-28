# getoptlong: testcase to demonstrate that getopt long is enumerable and takes
# its arguments
#
# The expected values are Ruby 3.4's, with getoptlong 0.2.1. Ruby takes the
# options out of ARGV; cameo leaves ARGV and gives the operands in rest.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "getoptlong"
opts = GetoptLong.new(["--name", "-n", GetoptLong::REQUIRED_ARGUMENT], ["--verbose", GetoptLong::NO_ARGUMENT], args: %w[-n Ann --verbose f1 f2])
p opts.reduce([]) { |acc, (opt, arg)| acc + [[opt, arg]] }, opts.rest, opts.to_a
p GetoptLong.new(["--x", GetoptLong::NO_ARGUMENT], args: %w[--x a]).map { |opt, _| opt }
p GetoptLong.new(["--x", GetoptLong::NO_ARGUMENT]).to_a, ARGV

__END__
[["--name", "Ann"], ["--verbose", ""]]
["f1", "f2"]
[]
["--x"]
[]
[]
