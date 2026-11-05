# Blocks that cameo build compiles to Go loops: times, upto and downto on
# an Integer, each on a Range of Integers, with break, next, return, a
# local of the block's own, and a parameter the body sets. A block
# captures the values of its variables, so a sum is a reduce, a count a
# count, which compile to the same loops with an accumulator. Every line
# prints what Ruby prints.

def sum(n)
  n.times.reduce(0) { |s, i| s + i }
end

def triangle(n)
  1.upto(n).reduce(0) { |t, i| t + i }
end

def countdown(n)
  n.downto(1).reduce(0) { |c, i| c * 10 + i }
end

def squares(lo, hi)
  (lo..hi).reduce(0) { |s, i| s + i * i }
end

def below(lo, hi)
  (lo...hi).reduce(0) { |c, _| c + 1 }
end

def first_multiple(n, of)
  0.upto(n) do |i|
    next if i == 0 || i % of != 0
    break i
  end
end

def own(n)
  n.times.reduce(0) do |s, i|
    sq = i * i
    s + sq
  end
end

def moved(n)
  n.times.reduce(0) do |s, i|
    i = i * 100
    s + i
  end
end

def early(n)
  n.times do |i|
    return i if i * i > n
  end
  -1
end

def receiver(n)
  m = n.times { }
  k = 1.upto(n) { }
  m + k
end

def wide(n)
  (n - 2).upto(n + 1).count { |i| i > n }
end

puts sum(100), triangle(100), countdown(5)
puts squares(1, 10), below(1, 10), below(5, 5), sum(0), sum(-3)
puts first_multiple(100, 7), first_multiple(5, 7)
puts own(10), moved(10), early(50), early(0)
puts receiver(4), wide(2**64)
