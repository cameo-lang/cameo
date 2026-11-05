package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby's, with forwardable 1.3.3 and delegate
// 0.4.0, but for __setobj__, which returns the copy here.

func TestForwardableAndDelegateForwardAsRubysDo(t *testing.T) {
	out := runScript(t, `require "forwardable"
require "delegate"
class Queue2
  extend Forwardable
  list :items, of: :int
  def_delegators :@items, :size, :first, :each, :include?
  def_delegator :@items, :last, :newest
  def_delegator :items, :length
  delegate [:min, :max] => :@items
  delegate sum: :@items
  def initialize(items)
    self.items = items
  end
end
q = Queue2.new([3, 1, 2])
p q.size, q.first, q.newest, q.length, q.min, q.max, q.sum, q.include?(2), q.each.to_a, q.each { |x| x }, Forwardable::VERSION
begin; q.map { |x| x }; rescue NoMethodError => e; puts e.message; end
module Reg
  extend SingleForwardable
  def_single_delegator :table, :keys, :names
  def_delegators :table, :size, :fetch
  def self.table
    {"a" => 1, "b" => 2}
  end
end
p Reg.names, Reg.size, Reg.fetch("b")
class W < SimpleDelegator
  def extra
    "extra:#{size}"
  end
end
w = W.new([1, 2, 3])
p w.size, w.first, w.extra, w.__getobj__, w == [1, 2, 3], w.is_a?(W), w.class, w.respond_to?(:size), w.respond_to?(:nope), W.ancestors.take(3)
p w.inspect, w.to_s, w.map { |x| x * 2 }, w.eql?([1, 2, 3]), (w <=> [1, 2]), w.__setobj__([9]).size, w.size
s = SimpleDelegator.new("str")
p s.upcase, s.length, s + "x", s.class, s.__getobj__, SimpleDelegator.new(nil).to_s, SimpleDelegator.new(5) + 1, SimpleDelegator.new(5) == 5, SimpleDelegator.new(5) == SimpleDelegator.new(5), SimpleDelegator.new(5) != 6
class Wrapped < DelegateClass(Array)
  def initialize(a)
    super(a)
  end
  def second
    self[1]
  end
end
ww = Wrapped.new([5, 6])
p ww.second, ww.size, ww.class, Wrapped.superclass.superclass, Delegator::VERSION
begin; w.nope; rescue NoMethodError => e; puts e.message; end
begin; Delegator.new(1); rescue TypeError => e; puts e.message; end
begin; SimpleDelegator.new; rescue ArgumentError => e; puts e.message; end
`)
	assert.Equals(t, out, `3
3
2
3
1
3
6
true
[3, 1, 2]
[3, 1, 2]
"1.3.3"
undefined method 'map' for an instance of Queue2
["a", "b"]
2
2
3
1
"extra:3"
[1, 2, 3]
true
true
W
true
false
[W, SimpleDelegator, Delegator]
"[1, 2, 3]"
"[1, 2, 3]"
[2, 4, 6]
true
1
1
3
"STR"
3
"strx"
SimpleDelegator
"str"
""
6
true
true
true
6
2
Wrapped
Delegator
"0.4.0"
undefined method 'nope' for an instance of W
Delegator is abstract: use SimpleDelegator or DelegateClass
wrong number of arguments (given 0, expected 1)
`)
}

// A delegator forwards to a target that answers through its own
// method_missing, another delegator for one; what Object defines is
// forwarded and the rest reaches the target through method_missing, so
// respond_to? answers for the target; freeze returns the delegator; the
// wrapped object counts in a delegator's identity as a key; a subclass's
// fields start as the core starts them and survive with. The answers are
// Ruby 3.4.1's.
func TestDelegatorsForwardThroughRespondToAndKeepTheirFields(t *testing.T) {
	out := runScript(t, `require "delegate"
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
`)
	assert.Equals(t, out, `wrong number of arguments (given 0, expected 1+)
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
`)
}
