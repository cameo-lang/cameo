s = Struct.new(*('a'..'z').map { |x| x.to_sym })
x = s.new

x[:z]

