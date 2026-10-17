first = 2**100
last = 2**1000
mid = (first + last) / 2
r = first..last

#--- benchmark: the prelude above runs once, the script below in the loop ---
r.bsearch { |x| x >= first }
