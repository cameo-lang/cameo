module SomeModule; end
class SimpleClass; end
class MediumClass
  10.times { include Module.new }
end
class LargeClass
  100.times { include Module.new }
end
class HugeClass
  300.times { include Module.new }
end
SimpleObj = SimpleClass.new
MediumObj = MediumClass.new
LargeObj = LargeClass.new
HugeObj = HugeClass.new

SomeModule === HugeObj

