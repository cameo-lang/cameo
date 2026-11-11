nums = (1..26).to_a
h = nums.each_with_object({}.compare_by_identity) { |n, h| h[n] = n }
200_000.times { nums.each { |n| h[n] } }
