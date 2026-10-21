# Integer methods that cameo build compiles to Go, and the calls they
# leave to the interpreter: an Integer beyond 64 bits, a Float argument, a
# division by zero. Every line prints what Ruby prints.

def fib(n)
  if n < 2
    n
  else
    fib(n - 1) + fib(n - 2)
  end
end

def gcd(a, b)
  t = 0
  while b != 0
    t = b
    b = a % b
    a = t
  end
  a
end

def power(base, exp)
  result = 1
  while exp > 0
    result *= base
    exp -= 1
  end
  result
end

def floor_div(a, b)
  a / b
end

def floor_mod(a, b)
  a % b
end

def sign(n)
  return -1 if n < 0
  return 1 if n > 0
  0
end

def within(n, lo, hi)
  if lo <= n && n <= hi || n == 0
    1
  else
    0
  end
end

def collatz(n)
  steps = 0
  until n == 1
    if n.even?
      n /= 2
    else
      n = 3 * n + 1
    end
    steps += 1
  end
  steps
end

def steps(n)
  count = 0
  until n == 1
    if n % 2 == 0
      n /= 2
    else
      n = 3 * n + 1
    end
    count += 1
  end
  count
end

def greet(name)
  "hello #{name}"
end

puts fib(30)
puts gcd(1071, 462)
puts power(2, 62)
puts power(2, 64)
puts power(2.5, 2)
puts floor_div(7, 2), floor_div(-7, 2), floor_div(7, -2)
puts floor_mod(7, 3), floor_mod(-7, 3), floor_mod(7, -3)
puts floor_div(2**64, 2**60)
puts sign(-5), sign(0), sign(9)
puts within(5, 1, 10), within(11, 1, 10), within(0, 1, 10)
puts collatz(27), steps(27)
puts greet("cameo")
begin
  floor_div(1, 0)
rescue ZeroDivisionError => e
  puts "#{e.class}: #{e.message}"
end
