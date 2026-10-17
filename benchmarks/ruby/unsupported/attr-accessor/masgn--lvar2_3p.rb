a = [nil] * 3
b = Class.new{attr_writer :a, :b, :c}.new
c = d = e = f = g = h = i = nil

(d, e = 1, 2, 3; nil)
