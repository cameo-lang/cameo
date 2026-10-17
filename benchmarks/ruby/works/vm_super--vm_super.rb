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

#--- benchmark: the prelude above runs once, the script below in the loop ---
obj.m
