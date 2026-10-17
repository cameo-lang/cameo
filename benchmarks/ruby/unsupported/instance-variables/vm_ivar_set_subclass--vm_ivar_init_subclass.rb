class A
  def set_ivars
    @a = nil
    @b = nil
    @c = nil
    @d = nil
    @e = nil
  end
end
class B < A; end
class C < A; end

b = B.new
c = C.new

b.set_ivars
c.set_ivars

