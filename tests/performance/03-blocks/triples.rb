# Pythagorean triples with sides up to 400, counted by three Range#each
# loops inside one another; prints the count and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def triples(n)
  count = 0
  (1..n).each do |a|
    (a..n).each do |b|
      (b..n).each do |c|
        count += 1 if a * a + b * b == c * c
      end
    end
  end
  count
end

t0 = now
r = triples(400)
puts "#{r} #{now - t0}"
