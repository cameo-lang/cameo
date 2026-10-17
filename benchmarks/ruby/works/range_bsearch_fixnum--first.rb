first = 1
last = 10000
mid = (first + last) / 2
r = first..last

#--- benchmark: the prelude above runs once, the script below in the loop ---
r.bsearch { |x| x >= first }
