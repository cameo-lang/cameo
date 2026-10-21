# The points of a 500 by 500 grid over the complex plane that stay bounded
# for 50 steps of z = z² + c, counted with Float arithmetic in Integer loops,
# as CRuby's app_mandelbrot without Complex; prints the count and the seconds
# it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def mandelbrot(size)
  inside = 0
  size.times do |y|
    ci = 2.0 * y / size - 1.0
    size.times do |x|
      cr = 2.0 * x / size - 1.5
      zr = 0.0
      zi = 0.0
      t = 0.0
      i = 0
      while i < 50 && zr * zr + zi * zi <= 4.0
        t = zr * zr - zi * zi + cr
        zi = 2.0 * zr * zi + ci
        zr = t
        i += 1
      end
      inside += 1 if i == 50
    end
  end
  inside
end

t0 = now
r = mandelbrot(500)
puts "#{r} #{now - t0}"
