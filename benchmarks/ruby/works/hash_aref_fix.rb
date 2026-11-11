nums = (1..26).to_a
h = nums.each_with_object({}) { |i, h| h[i] = i }
200_000.times { nums.each { |s| h[s] } }
