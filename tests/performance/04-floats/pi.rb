# Pi by the Leibniz series, ten million terms, a Float sum over an
# Integer loop; prints the result and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def pi(terms)
  terms.times.reduce(0.0) do |sum, k|
    sum + (k % 2 == 0 ? 4.0 : -4.0) / (2 * k + 1)
  end
end

t0 = now
r = pi(10_000_000)
puts "#{r} #{now - t0}"
