re = (1..)
rb = (..0)

#--- benchmark: the prelude above runs once, the script below in the loop ---
re.bsearch { |x| x >= 100000 }
