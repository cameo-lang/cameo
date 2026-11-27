# attributes: testcase to demonstrate the errors of class attributes a script
# can rescue
#
# Class attributes are cameo's own; these outputs are their specification. A
# class is built in its body as an instance is in initialize: self.name = v
# sets an attribute while the body runs, from the body or from a class method
# it calls, and the class is sealed when the body ends. What the checker
# refuses is checked in tests/attributes_test.go.
#
# (Introduced in PR #249. expected output follows the __END__ separator.)
class A
  attribute :x
  def self.set = self.y = 1
  begin
    set
  rescue NameError => e
    puts e.message
  end
  begin
    attribute :x
  rescue CameoError => e
    puts e.message
  end
  def self.x2 = 1
  begin
    attribute :x2
  rescue CameoError => e
    puts e.message
  end
end
p A.x

__END__
self.y = …: A has no attribute y
attribute x is already declared in A
attribute x2: A already defines a class method x2
nil
