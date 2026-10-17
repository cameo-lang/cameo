arr = [nil] * 30_000_000

#--- benchmark: the prelude above runs once, the script below in the loop ---
arr.each{|e|}
