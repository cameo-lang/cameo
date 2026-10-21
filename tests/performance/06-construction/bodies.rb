# Two bodies under a softened pull for a million steps, each step a new pair
# of bodies, since a body's fields are set in its construction alone; prints
# the squared distance at the end and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

class Body
  float :x, :y, :vx, :vy, :mass

  def initialize(x, y, vx, vy, mass)
    self.x = x
    self.y = y
    self.vx = vx
    self.vy = vy
    self.mass = mass
  end

  # pulled is the body one step later, of 1/per time units, under the
  # other's pull.
  def pulled(other, per)
    dt = 1.0 / per
    dx = other.x - x
    dy = other.y - y
    d2 = dx * dx + dy * dy + 0.01
    f = other.mass / (d2 * d2)
    nvx = vx + dx * f * dt
    nvy = vy + dy * f * dt
    Body.new(x + nvx * dt, y + nvy * dt, nvx, nvy, mass)
  end

  def dist2(other) = (x - other.x) * (x - other.x) + (y - other.y) * (y - other.y)
end

def orbit(steps)
  a = Body.new(0.0, 0.0, 0.0, 0.0, 10.0)
  b = Body.new(1.0, 0.0, 0.0, 3.0, 0.1)
  steps.times do
    na = a.pulled(b, 1000)
    nb = b.pulled(a, 1000)
    a = na
    b = nb
  end
  a.dist2(b)
end

t0 = now
r = orbit(1_000_000)
puts "#{r} #{now - t0}"
