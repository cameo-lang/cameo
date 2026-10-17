class C < Array
  def set_ivars
    @a = nil
    @b = nil
    @c = nil
  end
end

c = C.new

c.set_ivars

