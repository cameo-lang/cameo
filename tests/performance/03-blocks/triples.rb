# Pythagorean triples with sides up to 400, counted by three Range#each
# loops inside one another; prints the count and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def triples(n)
  (1..n).sum do |a|
    (a..n).sum do |b|
      (b..n).count { |c| a * a + b * b == c * c }
    end
  end
end

t0 = now
r = triples(400)
puts "#{r} #{now - t0}"
