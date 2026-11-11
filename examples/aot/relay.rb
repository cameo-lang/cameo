# A long computation in compiled methods, one calling the other: a host
# that pauses the run gets a dump of it, although the methods run as Go,
# and the resumed run finishes the computation.

def lap(n)
  i = 0
  total = 0
  while i < n
    i += 1
    total += i % 7
  end
  total
end

def relay(laps, n)
  total = 0
  done = 0
  while done < laps
    total += lap(n)
    done += 1
  end
  total
end

puts relay(20, 10_000_000)
