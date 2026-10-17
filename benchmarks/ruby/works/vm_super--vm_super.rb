class C
  def m
    1
  end
end

class CC < C
  def m
    super()
  end
end

obj = CC.new

obj.m
