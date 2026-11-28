# forwardable: testcase to demonstrate that delegators forward through respond
# to and keep their fields
#
# A delegator forwards to a target that answers through its own
# method_missing, another delegator for one; what Object defines is forwarded
# and the rest reaches the target through method_missing, so respond_to?
# answers for the target; freeze returns the delegator; the wrapped object
# counts in a delegator's identity as a key; a subclass's fields start as the
# core starts them and survive with. The answers are Ruby 3.4.1's.
#
# The expected strings are Ruby's, with forwardable 1.3.3 and delegate 0.4.0,
# but for __setobj__, which returns the copy here.
#
# (Introduced in PR #283. expected output follows the __END__ separator.)
require "delegate"
w = SimpleDelegator.new([1])
begin; w.method_missing; rescue ArgumentError => e; puts e.message; end
p SimpleDelegator.new(SimpleDelegator.new([1, 2])).size, SimpleDelegator.new(SimpleDelegator.new([1, 2])).respond_to?(:size)
p SimpleDelegator.new(5).respond_to?(:to_a), SimpleDelegator.new(5).respond_to?(:to_str), SimpleDelegator.new(5).respond_to?(:to_s), SimpleDelegator.new(5).respond_to?(:nope)
begin; SimpleDelegator.new(5).to_a; rescue NoMethodError => e; puts e.message; end
p w.freeze.class, w.frozen?
begin; SimpleDelegator.new(1).nope(2); rescue NoMethodError => e; p e.args, e.name, e.receiver.class; end
p({SimpleDelegator.new(1) => :a, SimpleDelegator.new(2) => :b}.size, [SimpleDelegator.new(1), SimpleDelegator.new(2)].uniq.size)
class Counted < SimpleDelegator
  int :count
  def initialize(o)
    super(o)
    self.count = 7
  end
end
c = Counted.new([1, 2])
p c.with(count: 9).size, c.with(count: 9).count
class Logged < SimpleDelegator
  list :log, of: :int
end
p Logged.new([1]).log

__END__
wrong number of arguments (given 0, expected 1+)
2
true
false
false
true
false
undefined method 'to_a' for an instance of SimpleDelegator
SimpleDelegator
true
[2]
:nope
SimpleDelegator
2
2
2
9
[]
