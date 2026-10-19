# async.rb without async: the recursion collects both parts in a list.
N = 32
def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
class FibNoAsync
  def of(n)
    return 1 if n < 2
    parts = []
    parts << self.of(n - 1)
    parts << self.of(n - 2)
    parts[0] + parts[1]
  end
end
t0 = now
r = FibNoAsync.new.of(N)
puts "#{r} #{now - t0}"
