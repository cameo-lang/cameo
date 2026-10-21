# Fibonacci by plain recursion, as CRuby's app_fib; prints the result and
# the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def fib(n)
  if n < 3
    1
  else
    fib(n - 1) + fib(n - 2)
  end
end

t0 = now
r = fib(34)
puts "#{r} #{now - t0}"
