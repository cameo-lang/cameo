# bigdecimal: testcase to demonstrate that numbers compare only with coercible
# values
#
# A number compares with a value that takes part in coerce, a BigDecimal, and
# with nothing else: Ruby's Integer#<=> answers nil for an object with a <=>
# of its own but no coerce.
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "bigdecimal"
class V
  def <=>(o); 1; end
end
p(1 <=> V.new, 1.5 <=> V.new, 1 <=> BigDecimal("2"), 3.0 <=> BigDecimal("2"), 1 == V.new)

__END__
nil
nil
-1
1
false
