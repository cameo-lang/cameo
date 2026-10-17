str = "a" * 1000
re = Regexp.new(str)

#--- benchmark: the prelude above runs once, the script below in the loop ---
Regexp.new(str)
