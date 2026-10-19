# async in the top ten levels, plain recursion below.
include Cameo
N = 32
def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
class Fib
  def of(n)
    return 1 if n < 2
    return seq(n) if n < N - 10
    parts = await([async(self, :of, n - 1), async(self, :of, n - 2)])
    parts[0] + parts[1]
  end

  def seq(n)
    return 1 if n < 2
    seq(n - 1) + seq(n - 2)
  end
end
t0 = now
r = Fib.new.of(N)
puts "#{r} #{now - t0}"
