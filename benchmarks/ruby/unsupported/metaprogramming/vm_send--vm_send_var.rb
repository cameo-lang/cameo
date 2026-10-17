class C
  def m
  end
end

o = C.new
m = :m

o.__send__ m

