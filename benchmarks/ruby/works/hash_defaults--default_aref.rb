h = Hash.new { :foo }

#--- benchmark: the prelude above runs once, the script below in the loop ---
h[1]
