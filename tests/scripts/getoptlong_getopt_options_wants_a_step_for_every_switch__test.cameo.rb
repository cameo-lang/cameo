# getoptlong: testcase to demonstrate that getopt options wants a step for
# every switch
#
# A switch declared without its step is refused at the declaration.
#
# The expected values are Ruby 3.4's, with getoptlong 0.2.1. Ruby takes the
# options out of ARGV; cameo leaves ARGV and gives the operands in rest.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "getoptlong"
begin
  class O < Getopt::Options
    bool :x, default: false
    on("--x")
  end
rescue ArgumentError => e
  puts e.message
end

__END__
no block given: on("--x") needs the step from the options so far to the next
