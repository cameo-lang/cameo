# Blocks that cameo build compiles to Go loops: times, upto and downto on
# an Integer, each on a Range of Integers, with break, next, return, a
# local of the block's own, and a parameter the body sets. Every line
# prints what Ruby prints.

def sum(n)
  s = 0
  n.times { |i| s += i }
  s
end

def triangle(n)
  t = 0
  1.upto(n) { |i| t += i }
  t
end

def countdown(n)
  c = 0
  n.downto(1) { |i| c = c * 10 + i }
  c
end

def squares(lo, hi)
  s = 0
  (lo..hi).each { |i| s += i * i }
  s
end

def below(lo, hi)
  c = 0
  (lo...hi).each { c += 1 }
  c
end

def first_multiple(n, of)
  found = 0
  1.upto(n) do |i|
    next if i % of != 0
    found = i
    break
  end
  found
end

def own(n)
  s = 0
  n.times do |i|
    sq = i * i
    s += sq
  end
  s
end

def moved(n)
  s = 0
  n.times do |i|
    i = i * 100
    s += i
  end
  s
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
  c = 0
  (n - 2).upto(n + 1) { |i| c += 1 if i > n }
  c
end

puts sum(100), triangle(100), countdown(5)
puts squares(1, 10), below(1, 10), below(5, 5), sum(0), sum(-3)
puts first_multiple(100, 7), first_multiple(5, 7)
puts own(10), moved(10), early(50), early(0)
puts receiver(4), wide(2**64)
