class C
  def method_missing mid
  end
end

obj = C.new

#--- benchmark: the prelude above runs once, the script below in the loop ---
obj.m; obj.m; obj.m; obj.m; obj.m; obj.m; obj.m; obj.m;

