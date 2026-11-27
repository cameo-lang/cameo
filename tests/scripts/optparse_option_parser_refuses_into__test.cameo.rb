# optparse: testcase to demonstrate that option parser refuses into
#
# The expected values are Ruby 3.4's, with optparse 0.6.0, but where a comment
# says otherwise.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "optparse"
begin
  OptionParser.new { |o| o.on("-v") }.parse(%w[-v], into: {})
rescue ArgumentError => e
  puts e.message
end

__END__
into: fills a Hash in place, which a value cannot be; read options after the parse, or use getopts, which returns the Hash
