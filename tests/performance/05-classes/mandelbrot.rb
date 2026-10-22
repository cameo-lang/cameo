# The Mandelbrot count of 04-floats as a method of a class whose typed
# fields hold the grid: the size and the step limit as ints, the corner and
# the scale as floats, read inside the loops; prints the count and the
# seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

class Grid
  int :size, :limit
  float :left, :top, :scale

  def initialize(size, limit, left, top, scale)
    self.size = size
    self.limit = limit
    self.left = left
    self.top = top
    self.scale = scale
  end

  def count
    inside = 0
    size.times do |y|
      ci = top + scale * y
      size.times do |x|
        cr = left + scale * x
        zr = 0.0
        zi = 0.0
        t = 0.0
        i = 0
        while i < limit && zr * zr + zi * zi <= 4.0
          t = zr * zr - zi * zi + cr
          zi = 2.0 * zr * zi + ci
          zr = t
          i += 1
        end
        inside += 1 if i == limit
      end
    end
    inside
  end
end

grid = Grid.new(500, 50, -1.5, -1.0, 2.0 / 500)
t0 = now
r = grid.count
puts "#{r} #{now - t0}"
