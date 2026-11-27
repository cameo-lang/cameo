# optparse: testcase to demonstrate that option parser records what it parsed
# in options
#
# The expected values are Ruby 3.4's, with optparse 0.6.0, but where a comment
# says otherwise.
#
# (Introduced in PR #288. expected output follows the __END__ separator.)
require "optparse"
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

__END__
{}
level high
["file"]
{verbose: true, name: "Ann", count: 3, x: true, level: :high}
[]
{verbose: false}
nil
1
