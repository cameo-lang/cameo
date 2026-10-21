# The sum of the Integers below thirty million with Integer#times, as
# CRuby's loop_times with a body; prints the result and the seconds it
# took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def sum(n)
  s = 0
  n.times { |i| s += i }
  s
end

t0 = now
r = sum(30_000_000)
puts "#{r} #{now - t0}"
