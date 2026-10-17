ios = 1000.times.map do
  100.times.map{IO.pipe}
end

# Process each batch of ios per iteration of the benchmark.
ios.pop.each do |r, w|
  r.close
  w.close
end

