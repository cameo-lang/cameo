# Takeuchi's function, as CRuby's app_tak; prints the result and the
# seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def tak(x, y, z)
  unless y < x
    z
  else
    tak(tak(x - 1, y, z), tak(y - 1, z, x), tak(z - 1, x, y))
  end
end

t0 = now
r = tak(18, 9, 0)
puts "#{r} #{now - t0}"
