arrays = (0..99).each_slice(10).to_a
#STDERR.puts arrays.inspect
h = arrays.each_with_object({}) { |s, acc| acc[s] = s }
200_000.times { arrays.each { |s| h[s] } }
