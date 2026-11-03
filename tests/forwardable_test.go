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
