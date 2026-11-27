# abbrev: testcase to demonstrate that abbrev refuses as Ruby does and scales
#
# A pattern that is neither a String nor a Regexp raises Ruby's NoMethodError
# when first used, so an empty list gives {}; keywords to Array#abbrev are the
# pattern, a Hash, as Ruby passes them; a surplus argument gets Array#abbrev's
# count; a class that includes the module has the method; three thousand words
# are abbreviated in one pass.
#
# The expected values are Ruby's, with abbrev 0.1.2.
#
# (Introduced in PR #281. expected output follows the __END__ separator.)
require "abbrev"
p Abbrev.abbrev([], 1)
begin; Abbrev.abbrev(%w[a], 1); rescue NoMethodError => e; puts e.message; end
begin; p %w[a].abbrev(x: 1); rescue NoMethodError => e; puts e.message; end
begin; %w[a].abbrev("x", "y"); rescue ArgumentError => e; puts e.message; end
class K
  include Abbrev
end
p K.new.abbrev(%w[xy])
words = (1..3000).map { |i| "w#{i.to_s.rjust(9, "0")}" }
p words.abbrev.size

__END__
{}
undefined method '=~' for an instance of Integer
undefined method '=~' for an instance of Hash
wrong number of arguments (given 2, expected 0..1)
{"xy" => "xy", "x" => "xy"}
3003
