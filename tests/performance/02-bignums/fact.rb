# Factorial of 5000, a hundred times, as CRuby's app_factorial: past 20!
# the product outgrows 64 bits, and every compiled call is left to the
# interpreter. Prints the digits of the result and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def fact(n)
  if n > 1
    n * fact(n - 1)
  else
    1
  end
end

t0 = now
r = 0
100.times { r = fact(5000) }
puts "#{r.to_s.size} #{now - t0}"
