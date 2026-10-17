floats = [*0.0.step(1.0, 0.0001)]

#--- benchmark: the prelude above runs once, the script below in the loop ---
floats.each {|f| f.to_s}
