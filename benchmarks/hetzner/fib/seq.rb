# Fibonacci by plain recursion; prints the result and the seconds it took.
N = 32
def now = Process.clock_gettime(Process::CLOCK_MONOTONIC)
class Fib
  def seq(n)
    return 1 if n < 2
    seq(n - 1) + seq(n - 2)
  end
end
t0 = now
r = Fib.new.seq(N)
puts "#{r} #{now - t0}"
