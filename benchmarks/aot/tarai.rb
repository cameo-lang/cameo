# Tarai, Takeuchi's function as he first wrote it, as CRuby's app_tarai;
# prints the result and the seconds it took.
def now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

def tarai(x, y, z)
  if x <= y
    y
  else
    tarai(tarai(x - 1, y, z), tarai(y - 1, z, x), tarai(z - 1, x, y))
  end
end

t0 = now
r = tarai(12, 6, 0)
puts "#{r} #{now - t0}"
