class SimpleClass; end
class OneModuleClass
  1.times { include Module.new }
end
class MediumClass
  10.times { include Module.new }
end
class LargeClass
  100.times { include Module.new }
end

Object.superclass

