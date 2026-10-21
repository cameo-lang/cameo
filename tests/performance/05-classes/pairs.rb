# Two vectors held by a pair through ref fields: the pair's method reads the
# vectors from its fields and calls their methods, which read theirs, ten
# million times; prints the sum and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

class Vec
  float :x, :y

  def initialize(x, y)
    self.x = x
    self.y = y
  end

  def dot(other) = x * other.x + y * other.y
  def dist2(other) = (x - other.x) * (x - other.x) + (y - other.y) * (y - other.y)
end

class Pair
  ref :a, Vec
  ref :b, Vec

  def initialize(a, b)
    self.a = a
    self.b = b
  end

  def churn(n)
    s = 0.0
    n.times { |i| s += a.dot(b) + a.dist2(b) / (i + 1) }
    s
  end
end

pair = Pair.new(Vec.new(1.5, 2.0), Vec.new(-0.5, 4.0))
t0 = now
r = pair.churn(10_000_000)
puts "#{r} #{now - t0}"
