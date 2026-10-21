# A loop that never ends, compiled: a host that stops the run stops it.

def spin(n)
  while n > 0
    n = n + 1 - 1
  end
  n
end

puts spin(1)
