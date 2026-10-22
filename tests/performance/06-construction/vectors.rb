# A walk of two million steps where every step builds a new vector: plus and
# scaled return Vec.new, so the loop constructs four million instances;
# prints the squared length reached and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

class Vec
  float :x, :y

  def initialize(x, y)
    self.x = x
    self.y = y
  end

  def plus(other) = Vec.new(x + other.x, y + other.y)
  def scaled(k) = Vec.new(x * k, y * k)
  def norm2 = x * x + y * y
end

def walk(n)
  v = Vec.new(0.0, 0.0)
  step = Vec.new(0.5, -0.25)
  n.times { |i| v = v.plus(step.scaled(i % 3)) }
  v.norm2
end

t0 = now
r = walk(2_000_000)
puts "#{r} #{now - t0}"
