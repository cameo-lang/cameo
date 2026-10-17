class Eight
  8.times { include(Module.new) }
end
class ThirtyTwo
  32.times { include(Module.new) }
end
class SixtyFour
  64.times { include(Module.new) }
end
class OneTwentyEight
  128.times { include(Module.new) }
end
class OnePositional
  def initialize a; end
end
class TwoPositional
  def initialize a, b; end
end
class ThreePositional
  def initialize a, b, c; end
end
class FourPositional
  def initialize a, b, c, d; end
end
class KWArg
  def initialize a:, b:, c:, d:
  end
end
class Mixed
  def initialize a, b, c:, d:
  end
end
# Disable GC to see raw throughput:
GC.disable

Eight.new
