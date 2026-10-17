class C < Array
  attr_reader :a, :b, :c
  def initialize
    @a = nil
    @b = nil
    @c = nil
  end
end

c = C.new

c.a
c.b
c.c

