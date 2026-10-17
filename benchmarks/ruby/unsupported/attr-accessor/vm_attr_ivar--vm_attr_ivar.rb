class C
  attr_reader :a, :b
  def initialize
    @a = nil
    @b = nil
  end
end
obj = C.new

j = obj.a
k = obj.b

