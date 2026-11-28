# getoptlong: testcase to demonstrate that getopt long constants and errors
#
# The expected values are Ruby 3.4's, with getoptlong 0.2.1. Ruby takes the
# options out of ARGV; cameo leaves ARGV and gives the operands in rest.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "getoptlong"
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
end

__END__
0
1
2
1
0
2
GetoptLong::Error
StandardError
"0.2.1"
option redefined `--help'
an invalid option `help'
no argument-flag
no argument-flag
nil
true
1
false
invoke ordering=, but option processing has already started
