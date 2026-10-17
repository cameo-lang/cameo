small_ary1 = [1, :a, nil, 2]
small_ary2 = [1, :a, nil, 2]
large_ary1 = Array.new(1000) { |i| i }
large_ary2 = Array.new(1000) { |i| i }
str_ary1 = Array.new(100) { |i| i.to_s }
str_ary2 = Array.new(100) { |i| i.to_s }

#--- benchmark: the prelude above runs once, the script below in the loop ---
large_ary1.eql?(large_ary2)
