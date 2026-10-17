class C
  @a = 1
  def self.a o
    @a = o; @a = o; @a = o; @a = o; @a = o; @a = o;
  end
end

a = C.a(nil)

