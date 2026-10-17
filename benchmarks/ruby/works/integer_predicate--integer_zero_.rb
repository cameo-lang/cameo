nums = (0..9).to_a

#--- benchmark: the prelude above runs once, the script below in the loop ---
nums.each { |n| n.zero? }
