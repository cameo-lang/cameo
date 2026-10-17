
s = Struct.new(*('a'..'z').map { |x| x.to_sym })
x = s.new
i = 0
while i<6_000_000
  i += 1
  x[:k] = i
end

