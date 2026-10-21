# Construction that cameo build compiles: Class.new of a class whose
# initialize sets fields from its parameters and does nothing else, and of a
# class without initialize, by keywords. The fields are converted and
# checked as initialize's assignments are: an Integer for a Float field
# becomes a Float, a default stands where no value is given, and a value
# that does not fit leaves the call to the interpreter, which raises. The
# expected output is the interpreter's: the field declarations are cameo's.

class Vec
  float :x, :y
  int :tag, default: 9

  def initialize(x, y)
    self.x = x
    self.y = y
  end

  def plus(other) = Vec.new(x + other.x, y + other.y)
  def scaled(k) = Vec.new(x * k, y * k)
  def norm2 = x * x + y * y
end

class Tagged
  int :n
  float :f
  bool :on, default: true
  ref :v, Vec
end

class Twice
  float :a
  int32 :small

  def initialize(a, small)
    self.a = a * 2
    self.small = small
  end
end

def walk(n)
  v = Vec.new(0, 0)
  step = Vec.new(0.5, -0.25)
  n.times { |i| v = v.plus(step.scaled(i % 3)) }
  v.norm2
end

def make(n)
  t = Tagged.new(n: n, f: n, v: Vec.new(1, 2))
  t.n + t.f + t.v.tag + (t.on ? 1 : 0)
end

def build(n) = Vec.new(n, n + 1)
def doubled(n) = Twice.new(n, 1).a
def shrink(n) = Twice.new(1, n).small

puts walk(10), make(4), build(3).x, build(3).tag, doubled(5), doubled(2**70), shrink(7)
p build(2), Tagged.new(n: 1)
begin
  shrink(2**40)
rescue RangeError => e
  puts e.message
end
