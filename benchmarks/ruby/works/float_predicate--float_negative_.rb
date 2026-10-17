floats = [1.0, -1.0, 0.0, Float::NAN, Float::INFINITY, -Float::INFINITY]

#--- benchmark: the prelude above runs once, the script below in the loop ---
floats.each { |f| f.negative? }
