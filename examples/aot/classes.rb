# Methods of classes with typed fields that cameo build compiles: a field
# read is a typed load from the instance, a parameter that has a field read
# on it is an instance of the class that declares the field, a call on an
# instance of a known class is a direct call, and a child class keeps its
# parent's fields where they were. Construction and field assignment stay
# with the interpreter. The expected output is the interpreter's: the field
# declarations are cameo's, not Ruby's.

class Vec
  float :x, :y

  def initialize(x, y)
    self.x = x
    self.y = y
  end

  def dot(other) = x * other.x + self.y * other.y
  def dist2(other) = (x - other.x) * (x - other.x) + (y - other.y) * (y - other.y)
  def norm2 = dot(self)
  def scaled(k) = norm2 * k
  def longer(other) = norm2 > other.norm2
end

class Vec3 < Vec
  float :z

  def initialize(x, y, z)
    self.x = x
    self.y = y
    self.z = z
  end

  def norm3 = norm2 + z * z
end

class Pair
  ref :a, Vec
  ref :b, Vec
  int :weight
  bool :flipped

  def initialize(a, b, weight, flipped: false)
    self.a = a
    self.b = b
    self.weight = weight
    self.flipped = flipped
  end

  def energy(n)
    s = 0.0
    n.times { |i| s += a.dot(b) * weight / (i + 1) }
    s
  end

  def sign = flipped ? -weight : weight
  def far(limit) = a.dist2(b) > limit && !flipped ? 1 : 0
end

class Counter
  int :start
  bignum :huge

  def initialize(start, huge)
    self.start = start
    self.huge = huge
  end

  def sum(n)
    s = start
    n.times { |i| s += i }
    s
  end

  def beyond = huge + start
end

def twice(v) = v.norm2 * 2

v = Vec.new(1.5, 2.0)
w = Vec3.new(1.0, 1.0, 3.0)
puts v.dot(w), v.norm2, w.norm3, v.scaled(2), w.scaled(2), v.longer(w), w.longer(v)
pair = Pair.new(v, w, 3)
puts pair.energy(4), pair.sign, Pair.new(w, v, 2, flipped: true).sign, pair.far(1), pair.far(100)
puts twice(v), twice(w)
c = Counter.new(10, 2**70)
puts c.sum(5), c.beyond, Counter.new(2**62, 1).sum(3)
