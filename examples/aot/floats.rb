# Floats that cameo build compiles: literals, locals, arithmetic with
# Integers, exact comparisons of the two, to_f and to_i, a method whose
# Float result comes through its recursion, and Integers past 64 bits beside
# Floats. Every line prints what Ruby prints.

def half(n)
  n / 2.0
end

def mean(n)
  s = 0.0
  1.upto(n) { |i| s += i }
  s / n
end

def ratio(a, b)
  return 0.0 if b == 0
  a.to_f / b
end

def trunc(n)
  (n * 0.75).to_i
end

def mixed(n)
  x = 2.5
  if n > x && n.to_f != x
    n % 2.5
  else
    -x
  end
end

def grows(n)
  return n.to_f if n > 3
  grows(n + 1) * 2
end

def remainders(n)
  (-7.5) % n + 7.5 % -n + n % 2.5 + n % -2.5
end

def exact(n)
  if n == 9007199254740993.0 || n.to_f != 9007199254740993.0
    1
  else
    0
  end
end

def steps(n)
  count = 0
  x = n.to_f
  while x > 1.0
    x = x / 1.5
    count += 1
  end
  count
end

puts half(7), mean(10), ratio(1, 3), ratio(1, 0), trunc(10), trunc(-10)
puts mixed(10), mixed(1), grows(0), remainders(2), remainders(4)
puts exact(9007199254740993), exact(9007199254740992), steps(1000), steps(0)
puts half(2**64), trunc(2**62), trunc(2**70), ratio(2**64, 3)
