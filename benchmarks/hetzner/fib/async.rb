# fib.rb's async version: async at every level.
include Cameo
N = 28
def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
class Fib
  def of(n)
    return 1 if n < 2
    promises = []
    promises << async(self, :of, n - 1)
    promises << async(self, :of, n - 2)
    parts = await(promises)
    parts[0] + parts[1]
  end
end
t0 = now
r = Fib.new.of(N)
puts "#{r} #{now - t0}"
