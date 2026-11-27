# attributes: testcase to demonstrate that reopening and class new set
# attributes too
#
# Class attributes are cameo's own; these outputs are their specification. A
# class is built in its body as an instance is in initialize: self.name = v
# sets an attribute while the body runs, from the body or from a class method
# it calls, and the class is sealed when the body ends.
#
# (Introduced in PR #249. expected output follows the __END__ separator.)
class Base
  attribute :names, default: []
  def self.add(n) = self.names += [n]
end
class Options < Base
  add(:a)
end
Before = Options
class Options
  attribute :names, default: []
  add(:b)
end
p Before.names, Options.names, Base.names
C = Class.new(Base) { add(:c) }
p C.names
D = Class.new(Base) do |me|
  me.names.size
end
p D.names

__END__
[:a]
[:a, :b]
[]
[:c]
[]
