class A0
  def method_missing(m); m end
end
class A1
  def method_missing(m, a) a; end
end
class S
  def method_missing(m, *a) a; end
end
class B
  def method_missing(m, kw: 1) kw end
end
class SB
  def method_missing(m, *a, kw: 1) kw end
end

t0 = 0.times.to_a
t1 = 1.times.to_a
t10 = 10.times.to_a
t200 = 200.times.to_a
kw = {kw: 2}

a0 = A0.new
a1 = A1.new
s = S.new
b = B.new
sb = SB.new

a0.()

