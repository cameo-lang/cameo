class C
  def method_missing mid
  end
end

obj = C.new

obj.m; obj.m; obj.m; obj.m; obj.m; obj.m; obj.m; obj.m;

