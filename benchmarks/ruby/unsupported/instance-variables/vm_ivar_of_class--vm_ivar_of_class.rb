class C
  @a = 1
  def self.a
    _a = @a; _a = @a; _a = @a; _a = @a; _a = @a;
    _a = @a; _a = @a; _a = @a; _a = @a; _a = @a;
  end
end

a = C.a

