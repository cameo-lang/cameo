a = [nil] * 3
b = Class.new{attr_writer :a, :b, :c}.new
c = d = e = f = g = h = i = nil

(a[0], a[1] = 1, 2; nil)
