a = [nil] * 3
b = Class.new{attr_writer :a, :b, :c}.new
c = d = e = f = g = h = i = nil

(b.a, b.b, b.c = g, h, i; nil)
