FLOATS = [*0.0.step(1.0, 0.001)]

#--- benchmark: the prelude above runs once, the script below in the loop ---
FLOATS.each {|f| f.to_s}

